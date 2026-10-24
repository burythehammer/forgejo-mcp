// SPDX-License-Identifier: GPL-3.0-or-later

// Package owner holds the owner read use case behind the forgejo://owner/{login}
// resource: a login names a user or, failing that, an organization. Like all of
// internal/app/ it is MCP-free and SDK-free (the depguard rules in .golangci.yml): it takes a
// typed login, reaches Forgejo only through Port, and returns repo-owned types
// from internal/app/model.
package owner

import (
	"context"
	"errors"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
)

// Port is the user and organization lookup the owner read needs.
// internal/forge/owneradapter implements it over forge.Users and forge.Orgs. A
// login Forgejo does not know is forge.ErrNotFound, wrapped or not; any other
// failure is an ordinary error for the use case to wrap.
type Port interface {
	GetUser(ctx context.Context, login string) (*model.User, error)
	GetOrg(ctx context.Context, name string) (*model.Organization, error)
}

// ErrOwnerNotFound is returned when the login is neither a user nor an
// organization. It is the owner read's own outcome, not a second not-found kind:
// the resource layer maps it to the owner-specific message.
var ErrOwnerNotFound = errors.New("owner not found")
