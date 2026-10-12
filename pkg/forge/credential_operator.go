// SPDX-License-Identifier: GPL-3.0-or-later

package forge

import "context"

// operatorKey marks a context whose request is served with the operator's own
// credential. It is unexported for the same reason tokenKey is.
const operatorKey contextKey = "forgejo-operator-credential"

// WithOperatorCredential marks ctx as served with the operator's own credential.
// The authentication port sets it when a request carries no credential of its
// own and the operator's credential may stand in for it. A Factory serves a
// marked context with its operator client, and refuses an unmarked one that
// carries no token.
//
// The mark is separate from the token because the operator's credential may
// legitimately be empty: an anonymous instance served over stdio sends no
// credential at all, and that must still be served rather than refused.
func WithOperatorCredential(ctx context.Context) context.Context {
	return context.WithValue(ctx, operatorKey, true)
}

// UsesOperatorCredential reports whether ctx was marked by WithOperatorCredential.
func UsesOperatorCredential(ctx context.Context) bool {
	marked, _ := ctx.Value(operatorKey).(bool)
	return marked
}
