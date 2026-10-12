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
// none. Whether that is refused, or served with the operator's own credential,
// is decided by the caller's policy, not by the port.
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
