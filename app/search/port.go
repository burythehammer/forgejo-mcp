// SPDX-License-Identifier: GPL-3.0-or-later

// Package search holds the search use cases behind the search tools. Like all
// of app/ it is MCP-free and SDK-free (scripts/ci/check-app-boundary.sh): use
// cases take typed inputs, reach Forgejo only through Port, and return
// repo-owned types from app/model for an MCP adapter in operation/search to
// render.
package search

import (
	"context"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
)

// Port is the search domain as the use cases need it, in repo-owned types.
// pkg/forge/searchadapter implements it over forge.Search.
type Port interface {
	SearchUsers(ctx context.Context, q UserQuery) ([]*model.User, error)
	SearchOrgTeams(ctx context.Context, org string, q TeamQuery) ([]*model.Team, error)
	SearchRepos(ctx context.Context, q RepoQuery) ([]*model.Repository, error)
}

// Paging selects a page of a search result.
type Paging struct{ Page, Limit int }

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
