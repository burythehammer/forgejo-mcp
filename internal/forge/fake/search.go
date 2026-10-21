// SPDX-License-Identifier: GPL-3.0-or-later

package fake

import (
	"context"
	"sort"
	"strings"
	"sync"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var _ forge.Search = (*Forgejo)(nil)

type searchState struct {
	mu    sync.Mutex
	users []*forgejo_sdk.User
	teams map[string][]*forgejo_sdk.Team // by organization
	repos []*forgejo_sdk.Repository
}

// SeedUser adds a user for SearchUsers to find.
func (f *Forgejo) SeedUser(u forgejo_sdk.User) {
	f.search.mu.Lock()
	defer f.search.mu.Unlock()
	f.search.users = append(f.search.users, &u)
}

// SeedTeam adds a team to an organization, which makes the organization exist
// for SearchOrgTeams.
func (f *Forgejo) SeedTeam(org string, t forgejo_sdk.Team) {
	f.search.mu.Lock()
	defer f.search.mu.Unlock()
	if f.search.teams == nil {
		f.search.teams = map[string][]*forgejo_sdk.Team{}
	}
	f.search.teams[org] = append(f.search.teams[org], &t)
}

// SeedSearchableRepo adds a repository for SearchRepos to find.
func (f *Forgejo) SeedSearchableRepo(r forgejo_sdk.Repository) {
	f.search.mu.Lock()
	defer f.search.mu.Unlock()
	f.search.repos = append(f.search.repos, &r)
}

func contains(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}

func (f *Forgejo) SearchUsers(_ context.Context, opt forgejo_sdk.SearchUsersOption) ([]*forgejo_sdk.User, error) {
	f.search.mu.Lock()
	defer f.search.mu.Unlock()
	out := []*forgejo_sdk.User{}
	for _, u := range f.search.users {
		if opt.KeyWord == "" || contains(u.UserName, opt.KeyWord) || contains(u.FullName, opt.KeyWord) || contains(u.Email, opt.KeyWord) {
			cp := *u
			out = append(out, &cp)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].UserName < out[j].UserName })
	lo, hi := page(opt.ListOptions, len(out))
	return out[lo:hi], nil
}

func (f *Forgejo) SearchOrgTeams(_ context.Context, org string, opt forgejo_sdk.SearchTeamsOptions) ([]*forgejo_sdk.Team, error) {
	f.search.mu.Lock()
	defer f.search.mu.Unlock()
	teams, ok := f.search.teams[org]
	if !ok {
		return nil, notFound("organization")
	}
	out := []*forgejo_sdk.Team{}
	for _, t := range teams {
		if opt.Query == "" || contains(t.Name, opt.Query) || (opt.IncludeDescription && contains(t.Description, opt.Query)) {
			cp := *t
			out = append(out, &cp)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	lo, hi := page(opt.ListOptions, len(out))
	return out[lo:hi], nil
}

func (f *Forgejo) SearchRepos(_ context.Context, opt forgejo_sdk.SearchRepoOptions) ([]*forgejo_sdk.Repository, error) {
	f.search.mu.Lock()
	defer f.search.mu.Unlock()
	out := []*forgejo_sdk.Repository{}
	for _, r := range f.search.repos {
		if opt.Keyword != "" && !contains(r.Name, opt.Keyword) && (!opt.KeywordInDescription || !contains(r.Description, opt.Keyword)) {
			continue
		}
		cp := *r
		out = append(out, &cp)
	}
	// Sort keys: alpha (default), updated, created, id. Order is asc unless
	// "desc" is asked for.
	less := func(i, j int) bool { return out[i].Name < out[j].Name }
	switch opt.Sort {
	case "updated":
		less = func(i, j int) bool { return out[i].Updated.Before(out[j].Updated) }
	case "created":
		less = func(i, j int) bool { return out[i].Created.Before(out[j].Created) }
	case "id":
		less = func(i, j int) bool { return out[i].ID < out[j].ID }
	}
	if opt.Order == "desc" {
		asc := less
		less = func(i, j int) bool { return asc(j, i) }
	}
	sort.SliceStable(out, less)
	lo, hi := page(opt.ListOptions, len(out))
	return out[lo:hi], nil
}
