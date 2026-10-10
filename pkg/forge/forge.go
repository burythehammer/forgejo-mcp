// SPDX-License-Identifier: GPL-3.0-or-later

// Package forge defines the Forgejo port: what the tools need from a Forgejo
// instance, expressed as interfaces with no SDK import. The real adapter lives
// in forge/sdk; an in-memory fake and a shared contract suite sit beside it.
//
// The port is split by domain. Each domain's interface is added here, and
// embedded in Forgejo, by the ticket that migrates that domain.
package forge

import "context"

// Forgejo is a port bound to one credential. It aggregates the per-domain
// ports; a domain joins it by embedding its interface here.
type Forgejo interface {
	Pulls
	BranchProtections
	Hooks
}

// Factory hands out a Forgejo port per request.
type Factory interface {
	// For applies the credential policy to ctx and returns a port bound to the
	// chosen credential: the request's own token if it carries one, otherwise
	// the operator's, where the policy allows that. When it does not, For
	// returns the policy's error.
	For(ctx context.Context) (Forgejo, error)
}
