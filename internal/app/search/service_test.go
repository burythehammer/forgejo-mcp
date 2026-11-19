// SPDX-License-Identifier: GPL-3.0-or-later

package search_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/list"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/model"
	appsearch "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/search"
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
	// max and known are the instance ceiling the test double reports.
	max   int
	known bool
}

func (p *stubPort) SearchUsers(_ context.Context, q appsearch.UserQuery) ([]*model.User, error) {
	p.queries = append(p.queries, q)
	return window(p.users, q.Paging), p.err
}

func (p *stubPort) SearchOrgTeams(_ context.Context, org string, q appsearch.TeamQuery) ([]*model.Team, error) {
	p.queries = append(p.queries, org, q)
	if teams, ok := p.teams[org]; ok {
		return window(teams, q.Paging), p.err
	}
	return nil, errors.New("organization not found")
}

func (p *stubPort) SearchRepos(_ context.Context, q appsearch.RepoQuery) ([]*model.Repository, error) {
	p.queries = append(p.queries, q)
	return window(p.repos, q.Paging), p.err
}

var page1 = appsearch.Paging{Page: 1, Limit: 100}

func TestUsersPassThroughTheQuery(t *testing.T) {
	t.Parallel()
	p := &stubPort{users: []*model.User{{UserName: "alice"}}}
	got, err := appsearch.New(p).Users(context.Background(), appsearch.UserQuery{Keyword: "al", Paging: page1})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 1 || got.Items[0].UserName != "alice" {
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
	if len(got.Items) != 1 || got.Items[0].Name != "devs" {
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
	if len(got.Items) != 2 || got.Items[0].Name != "tool-b" {
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

// MaxResponseItems reports the ceiling the test double was given.
func (p *stubPort) MaxResponseItems(context.Context) (int, bool) { return p.max, p.known }

// window returns the rows of all that page pg names, as Forgejo would.
func window[T any](all []T, pg appsearch.Paging) []T {
	lo := min((pg.Page-1)*pg.Limit, len(all))
	hi := min(lo+pg.Limit, len(all))
	return all[lo:hi]
}

func numberedUsers(n int) []*model.User {
	out := make([]*model.User, n)
	for i := range out {
		out[i] = &model.User{UserName: fmt.Sprintf("user-%d", i+1)}
	}
	return out
}

func TestUsersEnvelopeNamesTheNextPage(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p := &stubPort{users: numberedUsers(5)}
	first, err := appsearch.New(p).Users(ctx, appsearch.UserQuery{Paging: appsearch.Paging{Page: 1, Limit: 2}})
	if err != nil || len(first.Items) != 2 || first.Count != 2 || first.Page != 1 || first.Limit != 2 || !first.HasNext {
		t.Fatalf("page 1 of five: %+v, %v", first, err)
	}
	last, err := appsearch.New(p).Users(ctx, appsearch.UserQuery{Paging: appsearch.Paging{Page: 3, Limit: 2}})
	if err != nil || last.Count != 1 || last.HasNext {
		t.Fatalf("short last page: %+v, %v", last, err)
	}
	exact, err := appsearch.New(&stubPort{users: numberedUsers(4)}).Users(ctx, appsearch.UserQuery{Paging: appsearch.Paging{Page: 2, Limit: 2}})
	if err != nil || exact.Count != 2 || exact.HasNext {
		t.Fatalf("exact-multiple last page: %+v, %v", exact, err)
	}
	past, err := appsearch.New(&stubPort{users: numberedUsers(4)}).Users(ctx, appsearch.UserQuery{Paging: appsearch.Paging{Page: 3, Limit: 2}})
	if err != nil || past.Count != 0 || past.HasNext {
		t.Fatalf("past the end: %+v, %v", past, err)
	}
}

func TestReposEnvelopeHonoursTheKnownCeiling(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	repos := make([]*model.Repository, 5)
	for i := range repos {
		repos[i] = &model.Repository{Name: fmt.Sprintf("repo-%d", i+1)}
	}
	p := &stubPort{repos: repos, max: 3, known: true}
	got, err := appsearch.New(p).Repos(ctx, appsearch.RepoQuery{Paging: appsearch.Paging{Page: 1}})
	if err != nil || got.Limit != 3 || got.Count != 3 || !got.HasNext {
		t.Fatalf("default limit under ceiling: %+v, %v", got, err)
	}
	_, err = appsearch.New(p).Repos(ctx, appsearch.RepoQuery{Paging: appsearch.Paging{Page: 1, Limit: 4}})
	var over *list.OverCeilingError
	if !errors.As(err, &over) || over.Max != 3 {
		t.Fatalf("limit over ceiling: %v", err)
	}
}

func TestOrgTeamsEnvelopeNamesTheNextPage(t *testing.T) {
	t.Parallel()
	teams := []*model.Team{{Name: "a"}, {Name: "b"}, {Name: "c"}}
	p := &stubPort{teams: map[string][]*model.Team{"acme": teams}}
	got, err := appsearch.New(p).OrgTeams(context.Background(), "acme", appsearch.TeamQuery{Paging: appsearch.Paging{Page: 1, Limit: 2}})
	if err != nil || len(got.Items) != 2 || got.Count != 2 || !got.HasNext {
		t.Fatalf("page 1 of three: %+v, %v", got, err)
	}
}
