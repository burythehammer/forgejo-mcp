// SPDX-License-Identifier: GPL-3.0-or-later

package search_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
	appsearch "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/search"
)

// The use cases run against a stub port that holds fixed, repo-owned data and
// records the queries it was asked: no MCP, no SDK, no HTTP. Each test builds
// its own, so tests run in parallel.

type stubPort struct {
	users   []*model.User
	teams   map[string][]*model.Team
	repos   []*model.Repository
	err     error
	queries []any
}

func (p *stubPort) SearchUsers(_ context.Context, q appsearch.UserQuery) ([]*model.User, error) {
	p.queries = append(p.queries, q)
	return p.users, p.err
}

func (p *stubPort) SearchOrgTeams(_ context.Context, org string, q appsearch.TeamQuery) ([]*model.Team, error) {
	p.queries = append(p.queries, org, q)
	if teams, ok := p.teams[org]; ok {
		return teams, p.err
	}
	return nil, errors.New("organization not found")
}

func (p *stubPort) SearchRepos(_ context.Context, q appsearch.RepoQuery) ([]*model.Repository, error) {
	p.queries = append(p.queries, q)
	return p.repos, p.err
}

var page1 = appsearch.Paging{Page: 1, Limit: 100}

func TestUsersPassThroughTheQuery(t *testing.T) {
	t.Parallel()
	p := &stubPort{users: []*model.User{{UserName: "alice"}}}
	got, err := appsearch.New(p).Users(context.Background(), appsearch.UserQuery{Keyword: "al", Paging: page1})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].UserName != "alice" {
		t.Fatalf("got %+v", got)
	}
	if q := p.queries[0].(appsearch.UserQuery); q.Keyword != "al" || q.Page != 1 || q.Limit != 100 {
		t.Fatalf("query %+v", q)
	}
}

func TestUsersFailureIsWrapped(t *testing.T) {
	t.Parallel()
	p := &stubPort{err: errors.New("boom")}
	_, err := appsearch.New(p).Users(context.Background(), appsearch.UserQuery{Paging: page1})
	if err == nil || !strings.HasPrefix(err.Error(), "search user err: ") || !errors.Is(err, p.err) {
		t.Fatalf("got %v", err)
	}
}

func TestOrgTeamsAreAskedForTheNamedOrg(t *testing.T) {
	t.Parallel()
	p := &stubPort{teams: map[string][]*model.Team{"acme": {{Name: "devs"}}}}
	got, err := appsearch.New(p).OrgTeams(context.Background(), "acme", appsearch.TeamQuery{Keyword: "dev", Paging: page1})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "devs" {
		t.Fatalf("got %+v", got)
	}
}

func TestUnknownOrgIsWrapped(t *testing.T) {
	t.Parallel()
	p := &stubPort{teams: map[string][]*model.Team{}}
	_, err := appsearch.New(p).OrgTeams(context.Background(), "ghost", appsearch.TeamQuery{Paging: page1})
	if err == nil || !strings.HasPrefix(err.Error(), "search org teams err: ") {
		t.Fatalf("got %v", err)
	}
}

func TestReposPassThroughSortAndOrder(t *testing.T) {
	t.Parallel()
	p := &stubPort{repos: []*model.Repository{{Name: "tool-b"}, {Name: "tool-a"}}}
	got, err := appsearch.New(p).Repos(context.Background(), appsearch.RepoQuery{Keyword: "tool", Sort: "alpha", Order: "desc", Paging: page1})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "tool-b" {
		t.Fatalf("got %+v", got)
	}
	if q := p.queries[0].(appsearch.RepoQuery); q.Sort != "alpha" || q.Order != "desc" {
		t.Fatalf("query %+v", q)
	}
}

func TestReposFailureIsWrapped(t *testing.T) {
	t.Parallel()
	p := &stubPort{err: errors.New("boom")}
	_, err := appsearch.New(p).Repos(context.Background(), appsearch.RepoQuery{Paging: page1})
	if err == nil || !strings.HasPrefix(err.Error(), "search repos err: ") {
		t.Fatalf("got %v", err)
	}
}
