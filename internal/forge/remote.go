// SPDX-License-Identifier: GPL-3.0-or-later

package forge

import "context"

// remoteKey marks a request context served over a network transport.
type remoteKey struct{}

// MarkRemote returns ctx marked as served over a network transport (sse or
// http). A file written for such a request lands on the server host, where the
// caller cannot read it, so a tool that writes files refuses it. The transport
// marks every network request; stdio does not, and a call with no transport,
// such as the CLI, is not marked either.
func MarkRemote(ctx context.Context) context.Context {
	return context.WithValue(ctx, remoteKey{}, true)
}

// Remote reports whether ctx was marked by MarkRemote.
func Remote(ctx context.Context) bool {
	v, _ := ctx.Value(remoteKey{}).(bool)
	return v
}
