// SPDX-License-Identifier: GPL-3.0-or-later

package org

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/fake"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

// Handler tests run against the in-memory fake, build their own, and so run in
// parallel with no shared state.

type toolFn func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)

func newFixture() (handlers, *fake.Forgejo) {
	f := fake.New()
	return handlers{forge: fake.Factory{Forgejo: f}}, f
}

func req(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: args}}
}

// call runs a tool handler and fails the test unless it succeeded, returning
// the result text.
func call(t *testing.T, fn toolFn, args map[string]any) string {
	t.Helper()
	res, err := fn(context.Background(), req(args))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("err=%v res=%+v", err, res)
	}
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

// callErr runs a tool handler that must fail, returning its error.
func callErr(t *testing.T, fn toolFn, args map[string]any) error {
	t.Helper()
	res, err := fn(context.Background(), req(args))
	if err == nil {
		t.Fatalf("expected an error, got result %+v", res)
	}
	return err
}

// decodeResult unmarshals the {"Result": ...} envelope of a tool result.
func decodeResult(t *testing.T, text string, into any) {
	t.Helper()
	var env struct{ Result json.RawMessage }
	if err := json.Unmarshal([]byte(text), &env); err != nil {
		t.Fatalf("decoding %q: %v", text, err)
	}
	if err := json.Unmarshal(env.Result, into); err != nil {
		t.Fatalf("decoding result %s: %v", env.Result, err)
	}
}

func seedOrg(t *testing.T, f *fake.Forgejo, name string) {
	t.Helper()
	if _, err := f.CreateOrg(context.Background(), forgejo_sdk.CreateOrgOption{Name: name}); err != nil {
		t.Fatal(err)
	}
}

func TestCreateOrgFn(t *testing.T) {
	t.Parallel()
	h, f := newFixture()
	var got forgejo_sdk.Organization
	decodeResult(t, call(t, h.CreateOrgFn, map[string]any{
		"name": "acme", "full_name": "Acme Inc", "description": "anvils", "visibility": "private",
	}), &got)
	if got.UserName != "acme" {
		t.Fatalf("got %+v", got)
	}
	stored, err := f.GetOrg(context.Background(), "acme")
	if err != nil || stored.FullName != "Acme Inc" || stored.Description != "anvils" || stored.Visibility != "private" {
		t.Fatalf("stored %+v err=%v", stored, err)
	}
}

func TestCreateOrgFn_MissingName(t *testing.T) {
	t.Parallel()
	h, _ := newFixture()
	callErr(t, h.CreateOrgFn, map[string]any{})
}

func TestCreateOrgFn_DuplicateReportsForgejoError(t *testing.T) {
	t.Parallel()
	h, f := newFixture()
	seedOrg(t, f, "acme")
	err := callErr(t, h.CreateOrgFn, map[string]any{"name": "acme"})
	if !strings.HasPrefix(err.Error(), "create org err: ") {
		t.Fatalf("got %v", err)
	}
}

func TestGetOrgFn(t *testing.T) {
	t.Parallel()
	h, f := newFixture()
	seedOrg(t, f, "acme")
	var got forgejo_sdk.Organization
	decodeResult(t, call(t, h.GetOrgFn, map[string]any{"org": "acme"}), &got)
	if got.UserName != "acme" {
		t.Fatalf("got %+v", got)
	}
}

func TestGetOrgFn_MissingOrg(t *testing.T) {
	t.Parallel()
	h, _ := newFixture()
	callErr(t, h.GetOrgFn, map[string]any{})
}

func TestGetOrgFn_Unknown(t *testing.T) {
	t.Parallel()
	h, _ := newFixture()
	err := callErr(t, h.GetOrgFn, map[string]any{"org": "nothing"})
	if !strings.HasPrefix(err.Error(), "get org err: ") {
		t.Fatalf("got %v", err)
	}
}

func TestListMyOrgsFn(t *testing.T) {
	t.Parallel()
	h, f := newFixture()
	seedOrg(t, f, "a")
	seedOrg(t, f, "b")
	seedOrg(t, f, "c")
	var got []forgejo_sdk.Organization
	decodeResult(t, call(t, h.ListMyOrgsFn, map[string]any{"page": float64(2), "limit": float64(2)}), &got)
	if len(got) != 1 || got[0].UserName != "c" {
		t.Fatalf("page 2 = %+v", got)
	}
}

func TestListUserOrgsFn(t *testing.T) {
	t.Parallel()
	h, f := newFixture()
	seedOrg(t, f, "acme")
	seedOrg(t, f, "other")
	if err := f.AddOrgMember("acme", "bob"); err != nil {
		t.Fatal(err)
	}
	var got []forgejo_sdk.Organization
	decodeResult(t, call(t, h.ListUserOrgsFn, map[string]any{"user": "bob"}), &got)
	if len(got) != 1 || got[0].UserName != "acme" {
		t.Fatalf("got %+v", got)
	}
}

func TestListUserOrgsFn_MissingUser(t *testing.T) {
	t.Parallel()
	h, _ := newFixture()
	callErr(t, h.ListUserOrgsFn, map[string]any{})
}

func TestEditOrgFn_ReturnsTheUpdatedOrg(t *testing.T) {
	t.Parallel()
	h, f := newFixture()
	seedOrg(t, f, "acme")
	var got forgejo_sdk.Organization
	decodeResult(t, call(t, h.EditOrgFn, map[string]any{"org": "acme", "description": "updated"}), &got)
	if got.Description != "updated" {
		t.Fatalf("got %+v", got)
	}
}

func TestEditOrgFn_MissingOrg(t *testing.T) {
	t.Parallel()
	h, _ := newFixture()
	callErr(t, h.EditOrgFn, map[string]any{})
}

func TestEditOrgFn_Unknown(t *testing.T) {
	t.Parallel()
	h, _ := newFixture()
	err := callErr(t, h.EditOrgFn, map[string]any{"org": "nothing", "description": "x"})
	if !strings.HasPrefix(err.Error(), "edit org err: ") {
		t.Fatalf("got %v", err)
	}
}

func TestDeleteOrgFn(t *testing.T) {
	t.Parallel()
	h, f := newFixture()
	seedOrg(t, f, "acme")
	if got := call(t, h.DeleteOrgFn, map[string]any{"org": "acme"}); !strings.Contains(got, "Organization 'acme' deleted successfully") {
		t.Fatalf("got %s", got)
	}
	if _, err := f.GetOrg(context.Background(), "acme"); err == nil {
		t.Fatal("organization should be gone")
	}
}

func TestDeleteOrgFn_MissingOrg(t *testing.T) {
	t.Parallel()
	h, _ := newFixture()
	callErr(t, h.DeleteOrgFn, map[string]any{})
}

func TestListOrgMembersFn(t *testing.T) {
	t.Parallel()
	h, f := newFixture()
	seedOrg(t, f, "acme")
	if err := f.AddOrgMember("acme", "bob"); err != nil {
		t.Fatal(err)
	}
	var got []forgejo_sdk.User
	decodeResult(t, call(t, h.ListOrgMembersFn, map[string]any{"org": "acme"}), &got)
	if len(got) != 2 || got[1].UserName != "bob" {
		t.Fatalf("got %+v", got)
	}
}

func TestListOrgMembersFn_MissingOrg(t *testing.T) {
	t.Parallel()
	h, _ := newFixture()
	callErr(t, h.ListOrgMembersFn, map[string]any{})
}

func TestCheckOrgMembershipFn(t *testing.T) {
	t.Parallel()
	h, f := newFixture()
	seedOrg(t, f, "acme")
	if err := f.AddOrgMember("acme", "bob"); err != nil {
		t.Fatal(err)
	}
	for user, want := range map[string]bool{"bob": true, "outsider": false} {
		var got struct {
			Org      string `json:"org"`
			User     string `json:"user"`
			IsMember bool   `json:"is_member"`
		}
		decodeResult(t, call(t, h.CheckOrgMembershipFn, map[string]any{"org": "acme", "user": user}), &got)
		if got.Org != "acme" || got.User != user || got.IsMember != want {
			t.Fatalf("%s: got %+v", user, got)
		}
	}
}

func TestCheckOrgMembershipFn_MissingArguments(t *testing.T) {
	t.Parallel()
	h, _ := newFixture()
	callErr(t, h.CheckOrgMembershipFn, map[string]any{"user": "bob"})
	callErr(t, h.CheckOrgMembershipFn, map[string]any{"org": "acme"})
}

func TestRemoveOrgMemberFn(t *testing.T) {
	t.Parallel()
	h, f := newFixture()
	seedOrg(t, f, "acme")
	if err := f.AddOrgMember("acme", "bob"); err != nil {
		t.Fatal(err)
	}
	if got := call(t, h.RemoveOrgMemberFn, map[string]any{"org": "acme", "user": "bob"}); !strings.Contains(got, "User 'bob' removed from organization 'acme'") {
		t.Fatalf("got %s", got)
	}
	if is, _ := f.CheckOrgMembership(context.Background(), "acme", "bob"); is {
		t.Fatal("bob should have been removed")
	}
}

func TestRemoveOrgMemberFn_MissingArguments(t *testing.T) {
	t.Parallel()
	h, _ := newFixture()
	callErr(t, h.RemoveOrgMemberFn, map[string]any{"user": "bob"})
	callErr(t, h.RemoveOrgMemberFn, map[string]any{"org": "acme"})
}

func TestListOrgTeamsFn(t *testing.T) {
	t.Parallel()
	h, f := newFixture()
	seedOrg(t, f, "acme")
	if _, err := f.CreateTeam(context.Background(), "acme", forgejo_sdk.CreateTeamOption{Name: "devs", Permission: forgejo_sdk.AccessModeRead}); err != nil {
		t.Fatal(err)
	}
	var got []forgejo_sdk.Team
	decodeResult(t, call(t, h.ListOrgTeamsFn, map[string]any{"org": "acme"}), &got)
	var names []string
	for _, tm := range got {
		names = append(names, tm.Name)
	}
	if strings.Join(names, ",") != "Owners,devs" {
		t.Fatalf("teams %v", names)
	}
}

func TestListOrgTeamsFn_MissingOrg(t *testing.T) {
	t.Parallel()
	h, _ := newFixture()
	callErr(t, h.ListOrgTeamsFn, map[string]any{})
}

func TestCreateOrgTeamFn_DefaultsToReadPermission(t *testing.T) {
	t.Parallel()
	h, f := newFixture()
	seedOrg(t, f, "acme")
	var got forgejo_sdk.Team
	decodeResult(t, call(t, h.CreateOrgTeamFn, map[string]any{"org": "acme", "name": "devs", "description": "builders"}), &got)
	if got.Name != "devs" || got.Permission != forgejo_sdk.AccessModeRead || got.Description != "builders" {
		t.Fatalf("got %+v", got)
	}
}

func TestCreateOrgTeamFn_Options(t *testing.T) {
	t.Parallel()
	h, f := newFixture()
	seedOrg(t, f, "acme")
	var got forgejo_sdk.Team
	decodeResult(t, call(t, h.CreateOrgTeamFn, map[string]any{
		"org": "acme", "name": "admins", "permission": "admin",
		"can_create_org_repo": true, "includes_all_repositories": true,
	}), &got)
	if got.Permission != forgejo_sdk.AccessModeAdmin || !got.CanCreateOrgRepo || !got.IncludesAllRepositories {
		t.Fatalf("got %+v", got)
	}
}

func TestCreateOrgTeamFn_MissingArguments(t *testing.T) {
	t.Parallel()
	h, _ := newFixture()
	callErr(t, h.CreateOrgTeamFn, map[string]any{"name": "devs"})
	callErr(t, h.CreateOrgTeamFn, map[string]any{"org": "acme"})
}

func teamFixture(t *testing.T) (handlers, *fake.Forgejo, int64) {
	t.Helper()
	h, f := newFixture()
	seedOrg(t, f, "acme")
	f.AddUser("bob")
	team, err := f.CreateTeam(context.Background(), "acme", forgejo_sdk.CreateTeamOption{Name: "devs", Permission: forgejo_sdk.AccessModeRead})
	if err != nil {
		t.Fatal(err)
	}
	return h, f, team.ID
}

func TestTeamMemberTools(t *testing.T) {
	t.Parallel()
	h, _, id := teamFixture(t)
	args := map[string]any{"team_id": float64(id), "user": "bob"}
	if got := call(t, h.AddTeamMemberFn, args); !strings.Contains(got, "User 'bob' added to team") {
		t.Fatalf("got %s", got)
	}
	if got := call(t, h.RemoveTeamMemberFn, args); !strings.Contains(got, "User 'bob' removed from team") {
		t.Fatalf("got %s", got)
	}
}

func TestTeamMemberTools_UnknownTeamReportsForgejoError(t *testing.T) {
	t.Parallel()
	h, _, _ := teamFixture(t)
	err := callErr(t, h.AddTeamMemberFn, map[string]any{"team_id": float64(9999), "user": "bob"})
	if !strings.HasPrefix(err.Error(), "add team member err: ") {
		t.Fatalf("got %v", err)
	}
}

func TestTeamMemberTools_MissingArguments(t *testing.T) {
	t.Parallel()
	h, _, _ := teamFixture(t)
	callErr(t, h.AddTeamMemberFn, map[string]any{"user": "bob"})
	callErr(t, h.AddTeamMemberFn, map[string]any{"team_id": float64(1)})
	callErr(t, h.RemoveTeamMemberFn, map[string]any{"user": "bob"})
	callErr(t, h.RemoveTeamMemberFn, map[string]any{"team_id": float64(1)})
}

func TestTeamRepoTools(t *testing.T) {
	t.Parallel()
	h, _, id := teamFixture(t)
	args := map[string]any{"team_id": float64(id), "org": "acme", "repo": "anvil"}
	if got := call(t, h.AddTeamRepoFn, args); !strings.Contains(got, "Repository 'acme/anvil' added to team") {
		t.Fatalf("got %s", got)
	}
	if got := call(t, h.RemoveTeamRepoFn, args); !strings.Contains(got, "Repository 'acme/anvil' removed from team") {
		t.Fatalf("got %s", got)
	}
}

func TestTeamRepoTools_MissingArguments(t *testing.T) {
	t.Parallel()
	h, _, _ := teamFixture(t)
	callErr(t, h.AddTeamRepoFn, map[string]any{"org": "acme", "repo": "anvil"})
	callErr(t, h.AddTeamRepoFn, map[string]any{"team_id": float64(1), "repo": "anvil"})
	callErr(t, h.RemoveTeamRepoFn, map[string]any{"team_id": float64(1), "org": "acme"})
}
