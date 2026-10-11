// SPDX-License-Identifier: GPL-3.0-or-later

package forge

import (
	"context"
	"errors"
)

type contextKey string

// TokenContextKey carries a request's own Forgejo credential in a context.
const TokenContextKey contextKey = "forgejo-token"

// WithToken adds a request's Forgejo token to the context. A Factory uses it,
// in preference to the operator's own credential, to build the port for that
// request.
func WithToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, TokenContextKey, token)
}

// TokenFromContext returns the request's own token, or "" when it carries none.
func TokenFromContext(ctx context.Context) string {
	token, _ := ctx.Value(TokenContextKey).(string)
	return token
}

// ErrNoRequestToken is returned when a request carries no credential of its own
// and this server does not fall back to the operator's configured credential.
//
// The fallback exists for the stdio transport, where the client launched this
// process and there is no per-request identity to carry. On a network transport
// the same fallback serves an anonymous request with the operator's own
// credential, so the network transports switch it off.
var ErrNoRequestToken = errors.New(
	"request carries no Authorization header and this server does not fall back to its own credential; " +
		"send a per-request token",
)
