// SPDX-License-Identifier: GPL-3.0-or-later

package transport

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/jwtissuer"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/log"
)

const (
	// protectedResourceMetadataPath is the RFC 9728 well-known location.
	protectedResourceMetadataPath = "/.well-known/oauth-protected-resource"

	// issuerDocumentCacheControl lets proxies and Forgejo cache the issuer
	// documents briefly. Forgejo applies its own cache on top.
	issuerDocumentCacheControl = "public, max-age=300"
)

// handler builds the whole HTTP surface of resource-server mode around the MCP
// transport handler. Its refusal-log budget is its own; a listener that serves
// the handler passes its own budget to handlerWithRefusals instead.
func (rs *resourceServer) handler(mcp http.Handler) (http.Handler, error) {
	return rs.handlerWithRefusals(mcp, &refusalLimiter{})
}

// handlerWithRefusals is handler with the refusal-log budget for the protected
// route's debug lines supplied by the caller. Serve passes the listener's own
// budget, so the route's lines and the door's Host and Origin lines draw on one
// bound per listener.
//
// The public routes (protected resource metadata, the issuer's discovery
// document and key set) and the one protected route (the MCP endpoint) are
// matched exactly on the request path. http.ServeMux is deliberately not used:
// it answers an unclean path such as "/issuer/../mcp" with a redirect, and
// Forgejo refuses to follow redirects when it fetches the issuer documents.
// Every other path gets 404. Authentication wraps the protected route only, so
// the documents a caller needs before it has a token are reachable without one.
func (rs *resourceServer) handlerWithRefusals(mcp http.Handler, refusals *refusalLimiter) (http.Handler, error) {
	resourceURL, err := url.Parse(rs.resource)
	if err != nil {
		return nil, fmt.Errorf("resource %q: %w", rs.resource, err)
	}
	issuerURL, err := url.Parse(rs.issuer.URL())
	if err != nil {
		return nil, fmt.Errorf("issuer %q: %w", rs.issuer.URL(), err)
	}

	metadataPath := protectedResourceMetadataPath + strings.TrimSuffix(resourceURL.Path, "/")
	metadataURL := (&url.URL{Scheme: resourceURL.Scheme, Host: resourceURL.Host, Path: metadataPath}).String()
	metadata, err := rs.metadataDocument()
	if err != nil {
		return nil, err
	}

	routes := map[string]http.Handler{
		metadataPath:                             staticDocument(metadata, ""),
		protectedResourceMetadataPath:            staticDocument(metadata, ""),
		issuerURL.Path + jwtissuer.DiscoveryPath: staticDocument(rs.issuer.DiscoveryDocument(), issuerDocumentCacheControl),
		issuerURL.Path + jwtissuer.JWKSPath:      staticDocument(rs.issuer.JWKS(), issuerDocumentCacheControl),
		streamableHTTPEndpointPath:               rs.authenticate(mcp, bearerChallenge(metadataURL, rs.scopes), refusals),
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route, ok := routes[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		route.ServeHTTP(w, r)
	}), nil
}

// staticDocument serves a fixed JSON document to GET and HEAD.
func staticDocument(body []byte, cacheControl string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if cacheControl != "" {
			w.Header().Set("Cache-Control", cacheControl)
		}
		if r.Method == http.MethodHead {
			return
		}
		_, _ = w.Write(body)
	})
}

// authenticate is the protected route's HTTP wrapper around the authentication
// port. It writes the port's refusal, or answers a failure to mint with a 500,
// and otherwise hands the minted token to next through the request context.
//
// refusals bounds the debug lines for refused requests. Those values come from
// strangers, so they are rate-bounded like the door's lines, and they draw on
// the budget the caller supplied; see handlerWithRefusals.
func (rs *resourceServer) authenticate(next http.Handler, challenge string, refusals *refusalLimiter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth, err := authenticateResourceServer(r, rs, challenge)
		if err != nil {
			var refused *resourceRefusal
			if !errors.As(err, &refused) {
				log.Error("Cannot mint a Forgejo token", log.ErrorField(err))
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			refusals.write(log.Debug, refused.logMsg, refused.logFields...)
			refused.writeTo(w)
			return
		}
		log.Debug("Authenticated a request to the MCP endpoint",
			log.StringField("sub", truncateForLog(auth.subject)),
			log.StringField("forgejo_audience", truncateForLog(auth.audience)),
		)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), mintedTokenKey{}, string(auth.credential))))
	})
}
