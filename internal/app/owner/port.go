// SPDX-License-Identifier: GPL-3.0-or-later

// Package owner holds the owner read use case behind the forgejo://owner/{login}
// resource: a login names a user or, failing that, an organization. Like all of
// internal/app/ it is MCP-free and SDK-free (scripts/ci/check-app-boundary.sh): it takes a
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
// login Forgejo does not know is ErrNotFound, wrapped or not; any other failure
// is an ordinary error for the use case to wrap.
type Port interface {
	GetUser(ctx context.Context, login string) (*model.User, error)
	GetOrg(ctx context.Context, name string) (*model.Organization, error)
}

// ErrNotFound is what a Port returns, wrapped or not, when Forgejo has no user
// or organization of that name.
var ErrNotFound = errors.New("not found")

// ErrOwnerNotFound is returned when the login is neither a user nor an
// organization.
var ErrOwnerNotFound = errors.New("owner not found")
