// SPDX-License-Identifier: GPL-3.0-or-later

package org_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	apporg "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/org"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/orgadapter"
)

// The member list of an org that does not exist is the shared not-found
// outcome. The same list under an org that exists with nobody in it is empty.
// These run over the in-memory fake through the port bridge, not the stub port.

func fakeOrgService(t *testing.T) (*apporg.Service, *fake.Forgejo) {
	t.Helper()
	f := fake.New()
	return apporg.New(func(context.Context) (apporg.Port, error) { return orgadapter.New(f), nil }), f
}

func TestMembers_MissingOrgIsNotFound(t *testing.T) {
	t.Parallel()
	s, _ := fakeOrgService(t)
	_, err := s.Members(context.Background(), "nobody", apporg.Paging{Page: 1, Limit: 10})
	if !errors.Is(err, forge.ErrNotFound) {
		t.Fatalf("got %v, want forge.ErrNotFound", err)
	}
}

func TestMembers_EmptyOrgIsEmptyList(t *testing.T) {
	t.Parallel()
	s, _ := fakeOrgService(t)
	ctx := context.Background()
	if _, err := s.CreateOrg(ctx, apporg.CreateOrgInput{Name: "octo-org"}); err != nil {
		t.Fatalf("create org: %v", err)
	}
	// Forgejo makes the creator a member; remove them to get an org with nobody in it.
	if err := s.RemoveMember(ctx, "octo-org", fake.Poster); err != nil {
		t.Fatalf("remove creator: %v", err)
	}
	out, err := s.Members(ctx, "octo-org", apporg.Paging{Page: 1, Limit: 10})
	if err != nil {
		t.Fatalf("got %v, want an empty list", err)
	}
	if len(out.Items) != 0 {
		t.Fatalf("got %d members, want none", len(out.Items))
	}
}

func TestMembers_OrgWithAMemberListsIt(t *testing.T) {
	t.Parallel()
	s, f := fakeOrgService(t)
	if _, err := s.CreateOrg(context.Background(), apporg.CreateOrgInput{Name: "octo-org"}); err != nil {
		t.Fatalf("create org: %v", err)
	}
	if err := f.AddOrgMember("octo-org", "octo"); err != nil {
		t.Fatalf("add member: %v", err)
	}
	out, err := s.Members(context.Background(), "octo-org", apporg.Paging{Page: 1, Limit: 10})
	if err != nil {
		t.Fatalf("got %v, want one member", err)
	}
	names := make([]string, 0, len(out.Items))
	for _, m := range out.Items {
		names = append(names, m.UserName)
	}
	if !slices.Contains(names, "octo") {
		t.Fatalf("got members %v, want octo among them", names)
	}
}
