// SPDX-License-Identifier: GPL-3.0-or-later

package operation

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/config"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
)

// credential is the Forgejo credential an inbound request carries, as the
// authentication port hands it on. The empty value means the request carries
// none. authenticatePassthrough returns that empty value only when the operator's
// credential may stand in; attachPassthroughCredential then marks the request
// for it, and the Forgejo factory serves the mark.
type credential string

// errCredentialRequired refuses a request that carries no usable credential on
// a transport that does not fall back to the operator's own credential.
var errCredentialRequired = errors.New("request carries no usable Authorization header")

// authenticatePassthrough is the authentication port in passthrough mode. It
// takes the caller's own credential from the Authorization header.
//
// It refuses only when the Config does not allow the operator's credential to
// stand in for a missing one. The refusal is the same whether the request
// carries no header, a bare token, or an unrecognised scheme.
func authenticatePassthrough(r *http.Request, cfg config.Config) (credential, error) {
	if token := extractToken(r.Header.Get("Authorization")); token != "" {
		return credential(token), nil
	}
	if cfg.Listen.AllowOperatorTokenFallback {
		return "", nil
	}
	return "", errCredentialRequired
}

// attachCredential returns ctx carrying c for the handler. It is the only place
// a transport writes a request's credential into a context. An empty credential
// attaches nothing.
func attachCredential(ctx context.Context, c credential) context.Context {
	if c == "" {
		return ctx
	}
	return forge.WithToken(ctx, string(c))
}

// attachPassthroughCredential returns ctx as the handler of a passthrough
// request that authenticatePassthrough accepted sees it. A request that carried
// its own credential gets that credential. A request the port passed with none
// (the operator's credential is allowed to stand in) is marked for the
// operator's own credential instead, which is the whole of the fallback policy
// once it has left the Forgejo factory.
func attachPassthroughCredential(ctx context.Context, c credential) context.Context {
	if c == "" {
		return forge.WithOperatorCredential(ctx)
	}
	return forge.WithToken(ctx, string(c))
}

// attachStdioCredential returns ctx as a stdio request sees it. Stdio always
// falls back to the operator's credential: the client launched this process and
// there is no per-request identity to carry.
func attachStdioCredential(ctx context.Context) context.Context {
	return forge.WithOperatorCredential(ctx)
}

// extractToken returns the token from an Authorization header value, or "" when
// the value is absent, carries no scheme, or uses a scheme other than token or
// bearer.
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
