// SPDX-License-Identifier: GPL-3.0-or-later

package org_test

import (
	"context"
	"errors"
	"strings"
	"testing"

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

func (p *stubPort) ListMyOrgs(context.Context, apporg.Paging) ([]*model.Organization, error) {
	p.calls = append(p.calls, "mine")
	return nil, p.err
}

func (p *stubPort) ListUserOrgs(context.Context, string, apporg.Paging) ([]*model.Organization, error) {
	p.calls = append(p.calls, "user")
	return nil, p.err
}

func (p *stubPort) ListOrgMembers(context.Context, string, apporg.Paging) ([]*model.User, error) {
	p.calls = append(p.calls, "members")
	return nil, p.err
}

func (p *stubPort) CheckOrgMembership(context.Context, string, string) (bool, error) {
	p.calls = append(p.calls, "check")
	return true, p.err
}

func (p *stubPort) RemoveOrgMember(context.Context, string, string) error {
	p.calls = append(p.calls, "remove")
	return p.err
}

func (p *stubPort) ListOrgTeams(context.Context, string, apporg.Paging) ([]*model.Team, error) {
	p.calls = append(p.calls, "teams")
	return nil, p.err
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
		{"create", func(s *apporg.Service) error { _, err := s.CreateOrg(ctx, apporg.CreateOrgInput{Name: "o"}); return err }, "create org err: "},
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
