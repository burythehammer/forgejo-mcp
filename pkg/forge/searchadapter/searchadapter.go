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
	"encoding/json"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
	appsearch "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/search"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// New returns the use cases' port over s.
func New(s forge.Search) appsearch.Port { return adapter{s: s} }

type adapter struct{ s forge.Search }

var _ appsearch.Port = adapter{}

// convert maps an SDK value onto its repo-owned twin through JSON.
func convert[T any](from any, err error) (T, error) {
	var to T
	if err != nil {
		return to, err
	}
	b, err := json.Marshal(from)
	if err != nil {
		return to, err
	}
	err = json.Unmarshal(b, &to)
	return to, err
}

func listOptions(p appsearch.Paging) forgejo_sdk.ListOptions {
	return forgejo_sdk.ListOptions{Page: p.Page, PageSize: p.Limit}
}

func (a adapter) SearchUsers(ctx context.Context, q appsearch.UserQuery) ([]*model.User, error) {
	users, err := a.s.SearchUsers(ctx, forgejo_sdk.SearchUsersOption{KeyWord: q.Keyword, ListOptions: listOptions(q.Paging)})
	return convert[[]*model.User](users, err)
}

func (a adapter) SearchOrgTeams(ctx context.Context, org string, q appsearch.TeamQuery) ([]*model.Team, error) {
	teams, err := a.s.SearchOrgTeams(ctx, org, forgejo_sdk.SearchTeamsOptions{Query: q.Keyword, ListOptions: listOptions(q.Paging)})
	return convert[[]*model.Team](teams, err)
}

func (a adapter) SearchRepos(ctx context.Context, q appsearch.RepoQuery) ([]*model.Repository, error) {
	repos, err := a.s.SearchRepos(ctx, forgejo_sdk.SearchRepoOptions{
		Keyword: q.Keyword, Sort: q.Sort, Order: q.Order, ListOptions: listOptions(q.Paging),
	})
	return convert[[]*model.Repository](repos, err)
}
