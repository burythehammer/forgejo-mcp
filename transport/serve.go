// SPDX-License-Identifier: GPL-3.0-or-later

package transport

import (
	"context"
	"fmt"
	"net/http"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/config"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"

	"github.com/mark3labs/mcp-go/server"
)

// streamableHTTPEndpointPath is the library's own default. It is named here
// because the transport is mounted on a mux at this path rather than served as
// the root handler.
const streamableHTTPEndpointPath = config.MCPEndpointPath

// requestTokenContextFunc lifts a per-request credential out of the
// Authorization header and into the context the tool handlers see.
func requestTokenContextFunc(cfg config.Config) func(context.Context, *http.Request) context.Context {
	return func(ctx context.Context, r *http.Request) context.Context {
		return withRequestToken(cfg, ctx, r)
	}
}

func withRequestToken(cfg config.Config, ctx context.Context, r *http.Request) context.Context {
	if cfg.ResourceServerMode() {
		// The Authorization header carries the identity provider's token, which
		// must never reach Forgejo. The authentication layer has minted a Forgejo
		// token for this request; only that one enters the handler context.
		if token, ok := r.Context().Value(mintedTokenKey{}).(string); ok && token != "" {
			return attachCredential(ctx, credential(token))
		}
		return ctx
	}
	// A refusal carries no context mark here, so the factory refuses the
	// request. The door (guardRequests) has already refused it on a network
	// transport.
	cred, err := authenticatePassthrough(r, cfg)
	if err != nil {
		return ctx
	}
	return attachPassthroughCredential(ctx, cred)
}

// disableLibraryHostCheck reports whether the library's own loopback protection
// should be switched off.
//
// When the operator declares an explicit allow-list, this server's middleware
// owns the Host policy and is stricter than the library's, which accepts any
// loopback name. Leaving both on means a request arriving over loopback with a
// declared external name is refused by the library after our check has already
// accepted it — two policies disagreeing about one header. With no declared
// list the library's protection stays on as written.
func disableLibraryHostCheck(cfg config.Config) bool { return len(cfg.Listen.AllowedHosts) > 0 }

// newSSEHandler builds the SSE transport exactly as Serve does.
//
// It exists so the conformance tests drive the real construction — the option
// set, the context function and the mount — rather than a copy of it that can
// drift. A test that builds its own handler proves nothing about this one.
func newSSEHandler(s *server.MCPServer, cfg config.Config) http.Handler {
	opts := []server.SSEOption{server.WithSSEContextFunc(requestTokenContextFunc(cfg))}
	if disableLibraryHostCheck(cfg) {
		opts = append(opts, server.WithSSEDisableLocalhostProtection(true))
	}
	return server.NewSSEServer(s, opts...)
}

// newStreamableHTTPHandler builds the streamable HTTP transport exactly as Serve
// does, mounted on a mux at its endpoint path. Serving the transport as the
// root handler instead would make it answer on every path.
func newStreamableHTTPHandler(s *server.MCPServer, cfg config.Config) http.Handler {
	opts := []server.StreamableHTTPOption{
		server.WithHTTPContextFunc(requestTokenContextFunc(cfg)),
		server.WithEndpointPath(streamableHTTPEndpointPath),
	}
	if disableLibraryHostCheck(cfg) {
		opts = append(opts, server.WithDisableLocalhostProtection(true))
	}
	mux := http.NewServeMux()
	mux.Handle(streamableHTTPEndpointPath, server.NewStreamableHTTPServer(s, opts...))
	return mux
}

// Serve serves mcpServer on cfg.Transport: resource-server mode when configured,
// otherwise the Forgejo connection gate and then the transport cfg names. Every
// policy it applies is read from cfg.
func Serve(cfg config.Config, mcpServer *server.MCPServer, factory forge.Factory) error {
	transport := cfg.Transport
	if cfg.ResourceServerMode() {
		// config.Load has already refused every transport but http.
		rs, err := prepareResourceServer(context.Background(), cfg, factory)
		if err != nil {
			return err
		}
		// The listener is resolved before the handler, so the protected route's
		// refusal lines and the door's lines draw on one budget.
		listener := resolveTransportConfig(cfg)
		handler, err := rs.handlerWithRefusals(newStreamableHTTPHandler(mcpServer, cfg), listener.refusals)
		if err != nil {
			return fmt.Errorf("refusing to start: %w", err)
		}
		log.Info("Starting MCP streamable HTTP server in resource-server mode",
			log.IntField("port", cfg.Listen.HTTPPort),
			log.StringField("resource", rs.resource),
			log.StringField("authorization_server", rs.authServer),
		)
		if err := serveMCPOverHTTP("http", handler, cfg.Listen.HTTPPort, listener); err != nil {
			log.Error("Failed to start streamable HTTP server",
				log.IntField("port", cfg.Listen.HTTPPort),
				log.ErrorField(err),
			)
			return fmt.Errorf("failed to start streamable HTTP server: %w", err)
		}
		log.Info("MCP streamable HTTP server shutdown")
		return nil
	}

	// Test connection to Forgejo instance before starting the server
	log.Info("Testing connection to Forgejo instance",
		log.SanitizedURLField("url", cfg.Forgejo.URL),
	)
	if err := factory.VerifyConnection(context.Background()); err != nil {
		log.Error("Failed to connect to Forgejo instance",
			log.SanitizedURLField("url", cfg.Forgejo.URL),
			log.ErrorField(err),
		)
		return fmt.Errorf("connection test failed: %w", err)
	}
	log.Info("Successfully connected to Forgejo instance",
		log.SanitizedURLField("url", cfg.Forgejo.URL),
	)

	switch transport {
	case "stdio":
		log.Info("Starting MCP server with stdio transport")
		log.Info("MCP server ready for stdio communication")
		if err := server.ServeStdio(mcpServer, server.WithStdioContextFunc(attachStdioCredential)); err != nil {
			log.Error("MCP stdio server failed",
				log.ErrorField(err),
			)
			return err
		}
		log.Info("MCP stdio server shutdown")
	case "sse":
		sseServer := newSSEHandler(mcpServer, cfg)
		log.Info("Starting MCP SSE server",
			log.IntField("port", cfg.Listen.SSEPort),
		)
		if err := serveMCPOverHTTP("sse", sseServer, cfg.Listen.SSEPort, resolveTransportConfig(cfg)); err != nil {
			log.Error("Failed to start SSE server",
				log.IntField("port", cfg.Listen.SSEPort),
				log.ErrorField(err),
			)
			return fmt.Errorf("failed to start SSE server: %w", err)
		}
		log.Info("MCP SSE server shutdown")
	case "http":
		httpMux := newStreamableHTTPHandler(mcpServer, cfg)
		log.Info("Starting MCP streamable HTTP server",
			log.IntField("port", cfg.Listen.HTTPPort),
		)
		if err := serveMCPOverHTTP("http", httpMux, cfg.Listen.HTTPPort, resolveTransportConfig(cfg)); err != nil {
			log.Error("Failed to start streamable HTTP server",
				log.IntField("port", cfg.Listen.HTTPPort),
				log.ErrorField(err),
			)
			return fmt.Errorf("failed to start streamable HTTP server: %w", err)
		}
		log.Info("MCP streamable HTTP server shutdown")
	default:
		log.Error("Invalid transport configuration",
			log.StringField("transport", transport),
			log.StringField("valid_options", "stdio, sse, http"),
		)
		return fmt.Errorf("invalid transport type: %s. Must be 'stdio', 'sse', or 'http'", transport)
	}
	return nil
}
