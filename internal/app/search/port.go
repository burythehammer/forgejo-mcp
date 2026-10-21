// SPDX-License-Identifier: GPL-3.0-or-later

// Package search holds the search use cases behind the search tools. Like all
// of internal/app/ it is MCP-free and SDK-free (scripts/ci/check-app-boundary.sh): use
// cases take typed inputs, reach Forgejo only through Port, and return
// repo-owned types from internal/app/model for an MCP adapter in internal/operation/search to
// render.
package search

import (
	"context"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/list"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
)

// Port is the search domain as the use cases need it, in repo-owned types.
// internal/forge/searchadapter implements it over forge.Search.
type Port interface {
	// MaxResponseItems is the instance's pagination ceiling; ok is false when it
	// cannot be determined, which is "unknown", never a ceiling of zero.
	MaxResponseItems(ctx context.Context) (limit int, ok bool)
	SearchUsers(ctx context.Context, q UserQuery) ([]*model.User, error)
	SearchOrgTeams(ctx context.Context, org string, q TeamQuery) ([]*model.Team, error)
	SearchRepos(ctx context.Context, q RepoQuery) ([]*model.Repository, error)
}

// Paging selects a page of a search result. It is the list module's paging, so
// a search hands it to list.Window unchanged.
type Paging = list.Paging

// UserQuery searches users by keyword.
type UserQuery struct {
	Keyword string
	Paging
}

// TeamQuery searches the teams of one organization by keyword.
type TeamQuery struct {
	Keyword string
	Paging
}

// RepoQuery searches repositories by keyword, sorted and ordered as given.
type RepoQuery struct {
	Keyword, Sort, Order string
	Paging
}
