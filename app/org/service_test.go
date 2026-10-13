// SPDX-License-Identifier: GPL-3.0-or-later

package org_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/list"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
	apporg "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/org"
)

// The use cases run against a stub port that holds fixed, repo-owned data and
// records what it was asked: no MCP, no SDK, no HTTP. Each test builds its own,
// so tests run in parallel.

type stubPort struct {
	err      error
	orgName  string
	edited   apporg.EditOrgInput
	calls    []string
	team     apporg.CreateTeamInput
	getError error
	// orgs, members and teams are the rows the list stubs page through, and
	// max/known the instance ceiling they report.
	orgs    []*model.Organization
	members []*model.User
	teams   []*model.Team
	max     int
	known   bool
}

func (p *stubPort) CreateOrg(_ context.Context, in apporg.CreateOrgInput) (*model.Organization, error) {
	p.calls = append(p.calls, "create")
	return &model.Organization{UserName: in.Name, Visibility: in.Visibility}, p.err
}

func (p *stubPort) GetOrg(_ context.Context, org string) (*model.Organization, error) {
	p.calls = append(p.calls, "get")
	if p.getError != nil {
		return nil, p.getError
	}
	if p.err != nil {
		return nil, p.err
	}
	return &model.Organization{UserName: org, FullName: p.edited.FullName}, nil
}

func (p *stubPort) EditOrg(_ context.Context, org string, in apporg.EditOrgInput) error {
	p.calls = append(p.calls, "edit")
	p.orgName, p.edited = org, in
	return p.err
}

func (p *stubPort) DeleteOrg(context.Context, string) error {
	p.calls = append(p.calls, "delete")
	return p.err
}

func (p *stubPort) ListMyOrgs(_ context.Context, pg apporg.Paging) ([]*model.Organization, error) {
	p.calls = append(p.calls, "mine")
	return window(p.orgs, pg), p.err
}

func (p *stubPort) ListUserOrgs(_ context.Context, _ string, pg apporg.Paging) ([]*model.Organization, error) {
	p.calls = append(p.calls, "user")
	return window(p.orgs, pg), p.err
}

func (p *stubPort) ListOrgMembers(_ context.Context, _ string, pg apporg.Paging) ([]*model.User, error) {
	p.calls = append(p.calls, "members")
	return window(p.members, pg), p.err
}

func (p *stubPort) CheckOrgMembership(context.Context, string, string) (bool, error) {
	p.calls = append(p.calls, "check")
	return true, p.err
}

func (p *stubPort) RemoveOrgMember(context.Context, string, string) error {
	p.calls = append(p.calls, "remove")
	return p.err
}

func (p *stubPort) ListOrgTeams(_ context.Context, _ string, pg apporg.Paging) ([]*model.Team, error) {
	p.calls = append(p.calls, "teams")
	return window(p.teams, pg), p.err
}

func (p *stubPort) CreateTeam(_ context.Context, _ string, in apporg.CreateTeamInput) (*model.Team, error) {
	p.calls = append(p.calls, "create team")
	p.team = in
	return &model.Team{Name: in.Name, Permission: in.Permission}, p.err
}

func (p *stubPort) AddTeamMember(context.Context, int64, string) error {
	p.calls = append(p.calls, "add member")
	return p.err
}

func (p *stubPort) RemoveTeamMember(context.Context, int64, string) error {
	p.calls = append(p.calls, "remove member")
	return p.err
}

func (p *stubPort) AddTeamRepository(context.Context, int64, string, string) error {
	p.calls = append(p.calls, "add repo")
	return p.err
}

func (p *stubPort) RemoveTeamRepository(context.Context, int64, string, string) error {
	p.calls = append(p.calls, "remove repo")
	return p.err
}

// service returns the use cases over p.
func service(p *stubPort) *apporg.Service {
	return apporg.New(func(context.Context) (apporg.Port, error) { return p, nil })
}

var page1 = apporg.Paging{Page: 1, Limit: 100}

func TestTeamPermissionDefaultsToRead(t *testing.T) {
	t.Parallel()
	p := &stubPort{}
	if _, err := service(p).CreateTeam(context.Background(), "octo-org", apporg.CreateTeamInput{Name: "core"}); err != nil {
		t.Fatal(err)
	}
	if p.team.Permission != "read" {
		t.Fatalf("permission %q, want read", p.team.Permission)
	}
	if _, err := service(p).CreateTeam(context.Background(), "octo-org", apporg.CreateTeamInput{Name: "core", Permission: "admin"}); err != nil {
		t.Fatal(err)
	}
	if p.team.Permission != "admin" {
		t.Fatalf("permission %q, want admin", p.team.Permission)
	}
}

func TestEditReturnsTheOrgAsItNowIs(t *testing.T) {
	t.Parallel()
	p := &stubPort{}
	got, err := service(p).EditOrg(context.Background(), "octo-org", apporg.EditOrgInput{FullName: "New Name"})
	if err != nil {
		t.Fatal(err)
	}
	if got.FullName != "New Name" || strings.Join(p.calls, ",") != "edit,get" {
		t.Fatalf("got %+v after calls %v", got, p.calls)
	}
}

func TestEditStopsWhenTheRefetchFails(t *testing.T) {
	t.Parallel()
	p := &stubPort{getError: errors.New("boom")}
	_, err := service(p).EditOrg(context.Background(), "octo-org", apporg.EditOrgInput{})
	if err == nil || !strings.HasPrefix(err.Error(), "get updated org err: ") {
		t.Fatalf("got %v", err)
	}
}

func TestFailuresAreWrapped(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	ctx := context.Background()
	tests := []struct {
		name   string
		call   func(*apporg.Service) error
		prefix string
	}{
		{"create", func(s *apporg.Service) error {
			_, err := s.CreateOrg(ctx, apporg.CreateOrgInput{Name: "o"})
			return err
		}, "create org err: "},
		{"get", func(s *apporg.Service) error { _, err := s.GetOrg(ctx, "o"); return err }, "get org err: "},
		{"edit", func(s *apporg.Service) error { _, err := s.EditOrg(ctx, "o", apporg.EditOrgInput{}); return err }, "edit org err: "},
		{"delete", func(s *apporg.Service) error { return s.DeleteOrg(ctx, "o") }, "delete org err: "},
		{"my orgs", func(s *apporg.Service) error { _, err := s.MyOrgs(ctx, page1); return err }, "list my orgs err: "},
		{"user orgs", func(s *apporg.Service) error { _, err := s.UserOrgs(ctx, "u", page1); return err }, "list user orgs err: "},
		{"members", func(s *apporg.Service) error { _, err := s.Members(ctx, "o", page1); return err }, "list org members err: "},
		{"membership", func(s *apporg.Service) error { _, err := s.IsMember(ctx, "o", "u"); return err }, "check org membership err: "},
		{"remove member", func(s *apporg.Service) error { return s.RemoveMember(ctx, "o", "u") }, "remove org member err: "},
		{"teams", func(s *apporg.Service) error { _, err := s.Teams(ctx, "o", page1); return err }, "list org teams err: "},
		{"create team", func(s *apporg.Service) error { _, err := s.CreateTeam(ctx, "o", apporg.CreateTeamInput{}); return err }, "create org team err: "},
		{"add member", func(s *apporg.Service) error { return s.AddTeamMember(ctx, 1, "u") }, "add team member err: "},
		{"remove member from team", func(s *apporg.Service) error { return s.RemoveTeamMember(ctx, 1, "u") }, "remove team member err: "},
		{"add repo", func(s *apporg.Service) error { return s.AddTeamRepo(ctx, 1, "o", "r") }, "add team repo err: "},
		{"remove repo", func(s *apporg.Service) error { return s.RemoveTeamRepo(ctx, 1, "o", "r") }, "remove team repo err: "},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.call(service(&stubPort{err: boom}))
			if err == nil || !strings.HasPrefix(err.Error(), tc.prefix) || !errors.Is(err, boom) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

// MaxResponseItems reports the ceiling the test double was given.
func (p *stubPort) MaxResponseItems(context.Context) (int, bool) { return p.max, p.known }

// window returns the rows of all that page pg names, as Forgejo would.
func window[T any](all []T, pg apporg.Paging) []T {
	lo := min((pg.Page-1)*pg.Limit, len(all))
	hi := min(lo+pg.Limit, len(all))
	return all[lo:hi]
}

func numbered(n int) []*model.Organization {
	out := make([]*model.Organization, n)
	for i := range out {
		out[i] = &model.Organization{UserName: fmt.Sprintf("org-%d", i+1)}
	}
	return out
}

func TestListMyOrgsEnvelopeNamesTheNextPage(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p := &stubPort{orgs: numbered(5)}
	first, err := service(p).MyOrgs(ctx, apporg.Paging{Page: 1, Limit: 2})
	if err != nil || len(first.Organizations) != 2 || first.Count != 2 || first.Page != 1 || first.Limit != 2 || !first.HasNext {
		t.Fatalf("page 1 of five: %+v, %v", first, err)
	}
	last, err := service(p).MyOrgs(ctx, apporg.Paging{Page: 3, Limit: 2})
	if err != nil || last.Count != 1 || last.HasNext {
		t.Fatalf("short last page: %+v, %v", last, err)
	}
	exact, err := service(&stubPort{orgs: numbered(4)}).MyOrgs(ctx, apporg.Paging{Page: 2, Limit: 2})
	if err != nil || exact.Count != 2 || exact.HasNext {
		t.Fatalf("exact-multiple last page: %+v, %v", exact, err)
	}
	past, err := service(&stubPort{orgs: numbered(4)}).MyOrgs(ctx, apporg.Paging{Page: 3, Limit: 2})
	if err != nil || past.Count != 0 || past.HasNext {
		t.Fatalf("past the end: %+v, %v", past, err)
	}
}

func TestListMyOrgsPageZeroIsPageOne(t *testing.T) {
	t.Parallel()
	p := &stubPort{orgs: numbered(3)}
	got, err := service(p).MyOrgs(context.Background(), apporg.Paging{Page: 0, Limit: 2})
	if err != nil || got.Page != 1 || got.Count != 2 || !got.HasNext {
		t.Fatalf("page 0: %+v, %v", got, err)
	}
}

func TestListMyOrgsRefusesALimitOverAKnownCeiling(t *testing.T) {
	t.Parallel()
	p := &stubPort{orgs: numbered(5), max: 3, known: true}
	_, err := service(p).MyOrgs(context.Background(), apporg.Paging{Page: 1, Limit: 4})
	var over *list.OverCeilingError
	if !errors.As(err, &over) || over.Max != 3 {
		t.Fatalf("got %v", err)
	}
	got, err := service(p).MyOrgs(context.Background(), apporg.Paging{Page: 1})
	if err != nil || got.Limit != 3 || got.Count != 3 || !got.HasNext {
		t.Fatalf("default limit under ceiling: %+v, %v", got, err)
	}
}

func TestListMembersAndTeamsEnvelopeNameTheNextPage(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	members := make([]*model.User, 3)
	for i := range members {
		members[i] = &model.User{UserName: fmt.Sprintf("u%d", i+1)}
	}
	teams := make([]*model.Team, 3)
	for i := range teams {
		teams[i] = &model.Team{Name: fmt.Sprintf("t%d", i+1)}
	}
	p := &stubPort{members: members, teams: teams}
	m, err := service(p).Members(ctx, "o", apporg.Paging{Page: 1, Limit: 2})
	if err != nil || len(m.Members) != 2 || m.Count != 2 || !m.HasNext {
		t.Fatalf("members page 1: %+v, %v", m, err)
	}
	tm, err := service(p).Teams(ctx, "o", apporg.Paging{Page: 2, Limit: 2})
	if err != nil || len(tm.Teams) != 1 || tm.Count != 1 || tm.HasNext {
		t.Fatalf("teams page 2: %+v, %v", tm, err)
	}
}

func TestListUserOrgsEnvelopeNamesTheNextPage(t *testing.T) {
	t.Parallel()
	p := &stubPort{orgs: numbered(2)}
	got, err := service(p).UserOrgs(context.Background(), "u", apporg.Paging{Page: 1, Limit: 2})
	if err != nil || len(got.Organizations) != 2 || got.HasNext {
		t.Fatalf("exact two, limit two: %+v, %v", got, err)
	}
}
