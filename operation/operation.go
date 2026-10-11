package operation

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/actions"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/attachment"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/branchprotection"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/hook"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/issue"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/org"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/packages"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/pull"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/release"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/repo"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/search"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/tracking"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/user"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/version"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/wiki"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/config"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/sdk"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"
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
			return forgejo.WithToken(ctx, token)
		}
		return ctx
	}
	if token := extractToken(r.Header.Get("Authorization")); token != "" {
		return forgejo.WithToken(ctx, token)
	}
	return ctx
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

// newSSEHandler builds the SSE transport exactly as Run does.
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

// newStreamableHTTPHandler builds the streamable HTTP transport exactly as Run
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

// RegisterTool registers every tool; version is what the version tool reports.
// The Forgejo factory is built once by the composition root; a domain takes it
// when it migrates to the Forgejo port.
func RegisterTool(s *server.MCPServer, version string, f forge.Factory) {
	log.Info("Registering MCP tools")

	RegisterUserTool(s, f)
	RegisterRepoTool(s, f)
	RegisterIssueTool(s)
	RegisterPullTool(s, f)
	RegisterPullReviewTool(s, f)
	RegisterSearchTool(s, f)
	RegisterVersionTool(s, version)
	RegisterActionsTool(s, f)
	RegisterOrgTool(s, f)
	RegisterPackagesTool(s, f)
	RegisterTrackingTool(s, f)
	RegisterAttachmentTool(s, f)
	RegisterReleaseTool(s, f)
	RegisterBranchProtectionTool(s, f)
	RegisterHookTool(s, f)
	RegisterWikiTool(s, f)

	log.Info("All MCP tools registered successfully")
}

// Per-domain registration functions exposed for CLI domain grouping.

func RegisterUserTool(s *server.MCPServer, f forge.Factory) {
	user.RegisterTool(s, f)
	log.Debug("Registered user tools")
}

func RegisterRepoTool(s *server.MCPServer, f forge.Factory) {
	repo.RegisterTool(s, f)
	log.Debug("Registered repository tools")
}

func RegisterIssueTool(s *server.MCPServer) {
	issue.RegisterTool(s)
	log.Debug("Registered issue tools")
}

func RegisterPullTool(s *server.MCPServer, f forge.Factory) {
	pull.RegisterTool(s, f)
	log.Debug("Registered pull request tools")
}

func RegisterPullReviewTool(s *server.MCPServer, f forge.Factory) {
	pull.RegisterReviewTools(s, f)
	log.Debug("Registered pull review write tools")
}

func RegisterSearchTool(s *server.MCPServer, f forge.Factory) {
	search.RegisterTool(s, f)
	log.Debug("Registered search tools")
}

func RegisterVersionTool(s *server.MCPServer, ver string) {
	version.RegisterTool(s, ver)
	log.Debug("Registered version tools")
}

func RegisterActionsTool(s *server.MCPServer, f forge.Factory) {
	actions.RegisterTool(s, f)
	log.Debug("Registered actions tools")
}

func RegisterOrgTool(s *server.MCPServer, f forge.Factory) {
	org.RegisterTool(s, f)
	log.Debug("Registered org tools")
}

func RegisterPackagesTool(s *server.MCPServer, f forge.Factory) {
	packages.RegisterTool(s, f)
	log.Debug("Registered packages tools")
}

func RegisterTrackingTool(s *server.MCPServer, f forge.Factory) {
	tracking.RegisterTool(s, f)
	log.Debug("Registered time tracking tools")
}

func RegisterAttachmentTool(s *server.MCPServer, f forge.Factory) {
	attachment.RegisterTool(s, f)
	log.Debug("Registered attachment tools")
}

func RegisterReleaseTool(s *server.MCPServer, f forge.Factory) {
	release.RegisterTool(s, f)
	log.Debug("Registered release tools")
}

func RegisterBranchProtectionTool(s *server.MCPServer, f forge.Factory) {
	branchprotection.RegisterTool(s, f)
	log.Debug("Registered branch protection tools")
}

func RegisterHookTool(s *server.MCPServer, f forge.Factory) {
	hook.RegisterTools(s, f)
	log.Debug("Registered webhook tools")
}

func RegisterWikiTool(s *server.MCPServer, f forge.Factory) {
	wiki.RegisterTool(s, f)
	log.Debug("Registered wiki tools")
}

func extractToken(auth string) string {
	if auth == "" {
		return ""
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 {
		// No scheme prefix. Spec: bare tokens MUST be rejected and treated
		// as if no Authorization header were present (no silent identity).
		return ""
	}
	scheme := strings.ToLower(parts[0])
	if scheme == "token" || scheme == "bearer" {
		return parts[1]
	}
	return ""
}

// Run serves cfg on cfg.Transport. version is the build version, which is
// build information and so not part of cfg.
func Run(cfg config.Config, version string) error {
	transport := cfg.Transport
	mcpServer := newMCPServer(version)
	factory := sdk.NewFactory(cfg.Forgejo)
	RegisterTool(mcpServer, version, factory)
	RegisterCoreResources(mcpServer, factory)

	if cfg.ResourceServerMode() {
		// config.Load has already refused every transport but http.
		rs, err := prepareResourceServer(context.Background(), cfg)
		if err != nil {
			return err
		}
		handler, err := rs.handler(newStreamableHTTPHandler(mcpServer, cfg))
		if err != nil {
			return fmt.Errorf("refusing to start: %w", err)
		}
		log.Info("Starting MCP streamable HTTP server in resource-server mode",
			log.IntField("port", cfg.Listen.HTTPPort),
			log.StringField("resource", rs.resource),
			log.StringField("authorization_server", rs.authServer),
		)
		if err := serveMCPOverHTTP("http", handler, cfg.Listen.HTTPPort, cfg); err != nil {
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
	if err := testConnection(); err != nil {
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
		if err := server.ServeStdio(mcpServer); err != nil {
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
		if err := serveMCPOverHTTP("sse", sseServer, cfg.Listen.SSEPort, cfg); err != nil {
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
		if err := serveMCPOverHTTP("http", httpMux, cfg.Listen.HTTPPort, cfg); err != nil {
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

func testConnection() error {
	return forgejo.VerifyConnection()
}

func RegisterCoreResources(s *server.MCPServer, f forge.Factory) {
	RegisterCommitResource(s, f)
	RegisterIssueResources(s)
	RegisterLabelResources(s)
	RegisterOwnerResource(s, f)
	RegisterPullResources(s, f)
	RegisterRepoResource(s, f)
	RegisterStatusResource(s, f)
	RegisterBranchProtectionResources(s, f)
	RegisterHookResources(s, f)
	RegisterWikiResource(s, f)
	log.Debug("Registered core resource templates")
}

func RegisterWikiResource(s *server.MCPServer, f forge.Factory) {
	wiki.RegisterWikiResource(s, f)
	log.Debug("Registered wiki resource template")
}

func RegisterLabelResources(s *server.MCPServer) {
	issue.RegisterLabelResources(s)
	log.Debug("Registered label resource templates")
}

func RegisterBranchProtectionResources(s *server.MCPServer, f forge.Factory) {
	branchprotection.RegisterResource(s, f)
	log.Debug("Registered branch protection resource templates")
}

func RegisterHookResources(s *server.MCPServer, f forge.Factory) {
	hook.RegisterResources(s, f)
	log.Debug("Registered webhook resource templates")
}

func RegisterPullResources(s *server.MCPServer, f forge.Factory) {
	pull.RegisterPullResources(s, f)
	log.Debug("Registered pull request resource template")
}

func RegisterIssueResources(s *server.MCPServer) {
	issue.RegisterIssueResources(s)
	log.Debug("Registered issue resource templates")
}

func RegisterCommitResource(s *server.MCPServer, f forge.Factory) {
	repo.RegisterCommitResource(s, f)
	log.Debug("Registered commit resource template")
}

func RegisterOwnerResource(s *server.MCPServer, f forge.Factory) {
	user.RegisterOwnerResource(s, f)
	log.Debug("Registered owner resource template")
}

func RegisterRepoResource(s *server.MCPServer, f forge.Factory) {
	repo.RegisterRepoResource(s, f)
	log.Debug("Registered repo resource template")
}

func RegisterStatusResource(s *server.MCPServer, f forge.Factory) {
	repo.RegisterStatusResource(s, f)
	log.Debug("Registered commit status resource template")
}

func newMCPServer(version string) *server.MCPServer {
	return server.NewMCPServer(
		"Forgejo MCP Server",
		version,
		server.WithLogging(),
		server.WithResourceCapabilities(false, false),
	)
}
