// SPDX-License-Identifier: GPL-3.0-or-later

package forge

import (
	"context"
)

type contextKey string

// tokenKey carries a request's own Forgejo credential in a context. It is
// unexported so that WithToken and TokenFromContext are the only way in or out.
const tokenKey contextKey = "forgejo-token"

// WithToken adds a request's Forgejo token to the context. A Factory builds the
// port for that request from it, in preference to any operator mark.
func WithToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, tokenKey, token)
}

// TokenFromContext returns the request's own token, or "" when it carries none.
func TokenFromContext(ctx context.Context) string {
	token, _ := ctx.Value(tokenKey).(string)
	return token
}

// ErrNoRequestToken is returned when a request carries neither a credential of
// its own nor the mark for the operator's credential (WithOperatorCredential).
// Whether a request is marked is the authentication port's decision. The port
// marks stdio requests, and network requests only when the operator has opted in
// to the fallback.
var ErrNoRequestToken error = noRequestTokenError{}

// noRequestTokenError is a refused credential: it is an unauthorized kind, so
// renderers report it as access denied like any other 401.
type noRequestTokenError struct{}

func (noRequestTokenError) Error() string {
	return "request carries no Authorization header and this server does not fall back to its own credential; " +
		"send a per-request token"
}

func (noRequestTokenError) Is(target error) bool { return target == ErrUnauthorized }
