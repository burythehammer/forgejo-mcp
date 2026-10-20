// SPDX-License-Identifier: GPL-3.0-or-later

// Package searchadapter implements the search use cases' port (app/search.Port)
// over the Forgejo port (forge.Search). It is the one place that translates
// between repo-owned types and SDK types for the search domain: inputs become
// SDK options, SDK results become app/model values. Results are converted
// through their JSON form, which is exactly what a tool renders, so a field
// the repo-owned type shares by JSON name cannot drift in value.
package searchadapter

import (
	"context"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/model"
	appsearch "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/search"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge/bridge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// New returns the use cases' port over s.
// Source is the part of the Forgejo port the search domain uses: its search
// methods and the instance's pagination ceiling.
type Source interface {
	forge.Search
	forge.Ceilings
}

func New(s Source) appsearch.Port { return adapter{s: s} }

// MaxResponseItems reads the instance's pagination ceiling through the source.
func (a adapter) MaxResponseItems(ctx context.Context) (int, bool) {
	return a.s.MaxResponseItems(ctx)
}

type adapter struct{ s Source }

var _ appsearch.Port = adapter{}

func listOptions(p appsearch.Paging) forgejo_sdk.ListOptions {
	return forgejo_sdk.ListOptions{Page: p.Page, PageSize: p.Limit}
}

func (a adapter) SearchUsers(ctx context.Context, q appsearch.UserQuery) ([]*model.User, error) {
	users, err := a.s.SearchUsers(ctx, forgejo_sdk.SearchUsersOption{KeyWord: q.Keyword, ListOptions: listOptions(q.Paging)})
	return bridge.Convert[[]*model.User](users, err)
}

func (a adapter) SearchOrgTeams(ctx context.Context, org string, q appsearch.TeamQuery) ([]*model.Team, error) {
	teams, err := a.s.SearchOrgTeams(ctx, org, forgejo_sdk.SearchTeamsOptions{Query: q.Keyword, ListOptions: listOptions(q.Paging)})
	return bridge.Convert[[]*model.Team](teams, err)
}

func (a adapter) SearchRepos(ctx context.Context, q appsearch.RepoQuery) ([]*model.Repository, error) {
	repos, err := a.s.SearchRepos(ctx, forgejo_sdk.SearchRepoOptions{
		Keyword: q.Keyword, Sort: q.Sort, Order: q.Order, ListOptions: listOptions(q.Paging),
	})
	return bridge.Convert[[]*model.Repository](repos, err)
}
