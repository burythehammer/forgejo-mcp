// SPDX-License-Identifier: GPL-3.0-or-later

package org

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcptest"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// These tests pin the wire contract of every org, membership and team tool: its
// tool definition (schema snapshot) and its output and error text (golden),
// together with the calls the tool made on the Forgejo port. They run against a
// stub port that returns fixed, fully populated data, so the bytes are
// deterministic.

var allTools = []mcp.Tool{
	CreateOrgTool, GetOrgTool, ListMyOrgsTool, ListUserOrgsTool, EditOrgTool, DeleteOrgTool,
	ListOrgMembersTool, CheckOrgMembershipTool, RemoveOrgMemberTool,
	ListOrgTeamsTool, CreateOrgTeamTool, AddTeamMemberTool, RemoveTeamMemberTool, AddTeamRepoTool, RemoveTeamRepoTool,
}

func TestSchemaSnapshots(t *testing.T) {
	t.Parallel()
	for _, tool := range allTools {
		t.Run(tool.Name, func(t *testing.T) { mcptest.SchemaSnapshot(t, tool) })
	}
}

var (
	t0   = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	t1   = time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	user = &forgejo_sdk.User{ID: 1, UserName: "octo", FullName: "Octo Cat", Email: "octo@example.com", Created: t0, LastLogin: t1, IsActive: true}
)

func fixedOrg(name string) *forgejo_sdk.Organization {
	return &forgejo_sdk.Organization{ID: 6, UserName: name, FullName: "Octo Org", AvatarURL: "https://f.example/a.png", Description: "d", Website: "https://o.example", Location: "here", Visibility: "public"}
}

func fixedTeam(id int64) *forgejo_sdk.Team {
	return &forgejo_sdk.Team{ID: id, Name: "core", Description: "the core", Organization: fixedOrg("octo-org"), Permission: forgejo_sdk.AccessModeWrite, CanCreateOrgRepo: true, IncludesAllRepositories: false}
}

// stubOrgs is a forge.Forgejo whose org domain returns fixed data and records
// the calls it receives. fails maps a method name to the error it returns.
// Any other domain would panic on the nil embed.
type stubOrgs struct {
	forge.Forgejo
	mu    sync.Mutex
	calls []string
	fails map[string]error
}

func (s *stubOrgs) rec(name string, args ...any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, _ := json.Marshal(args)
	s.calls = append(s.calls, name+" "+string(b))
	return s.fails[name]
}

func (s *stubOrgs) CreateOrg(_ context.Context, opt forgejo_sdk.CreateOrgOption) (*forgejo_sdk.Organization, error) {
	return fixedOrg(opt.Name), s.rec("CreateOrg", opt)
}

func (s *stubOrgs) GetOrg(_ context.Context, org string) (*forgejo_sdk.Organization, error) {
	return fixedOrg(org), s.rec("GetOrg", org)
}

func (s *stubOrgs) EditOrg(_ context.Context, org string, opt forgejo_sdk.EditOrgOption) error {
	return s.rec("EditOrg", org, opt)
}

func (s *stubOrgs) DeleteOrg(_ context.Context, org string) error {
	return s.rec("DeleteOrg", org)
}

func (s *stubOrgs) ListMyOrgs(_ context.Context, opt forgejo_sdk.ListOrgsOptions) ([]*forgejo_sdk.Organization, error) {
	return []*forgejo_sdk.Organization{fixedOrg("one"), fixedOrg("two")}, s.rec("ListMyOrgs", opt)
}

func (s *stubOrgs) ListUserOrgs(_ context.Context, u string, opt forgejo_sdk.ListOrgsOptions) ([]*forgejo_sdk.Organization, error) {
	return []*forgejo_sdk.Organization{fixedOrg("three")}, s.rec("ListUserOrgs", u, opt)
}

func (s *stubOrgs) ListOrgMembers(_ context.Context, org string, opt forgejo_sdk.ListOrgMembershipOption) ([]*forgejo_sdk.User, error) {
	return []*forgejo_sdk.User{user}, s.rec("ListOrgMembers", org, opt)
}

func (s *stubOrgs) CheckOrgMembership(_ context.Context, org, u string) (bool, error) {
	return u == "octo", s.rec("CheckOrgMembership", org, u)
}

func (s *stubOrgs) RemoveOrgMember(_ context.Context, org, u string) error {
	return s.rec("RemoveOrgMember", org, u)
}

func (s *stubOrgs) ListOrgTeams(_ context.Context, org string, opt forgejo_sdk.ListTeamsOptions) ([]*forgejo_sdk.Team, error) {
	return []*forgejo_sdk.Team{fixedTeam(5)}, s.rec("ListOrgTeams", org, opt)
}

func (s *stubOrgs) CreateTeam(_ context.Context, org string, opt forgejo_sdk.CreateTeamOption) (*forgejo_sdk.Team, error) {
	return fixedTeam(7), s.rec("CreateTeam", org, opt)
}

func (s *stubOrgs) AddTeamMember(_ context.Context, teamID int64, u string) error {
	return s.rec("AddTeamMember", teamID, u)
}

func (s *stubOrgs) RemoveTeamMember(_ context.Context, teamID int64, u string) error {
	return s.rec("RemoveTeamMember", teamID, u)
}

func (s *stubOrgs) AddTeamRepository(_ context.Context, teamID int64, org, repo string) error {
	return s.rec("AddTeamRepository", teamID, org, repo)
}

func (s *stubOrgs) RemoveTeamRepository(_ context.Context, teamID int64, org, repo string) error {
	return s.rec("RemoveTeamRepository", teamID, org, repo)
}

// invoke calls a registered tool the way the server does and renders what the
// client would see: the text, or the error, then the port calls made. A
// non-nil credErr makes the credential policy refuse the request.
func invoke(t *testing.T, stub *stubOrgs, credErr error, tool string, args map[string]any) string {
	t.Helper()
	s := server.NewMCPServer("test", "0")
	RegisterTool(s, fake.Factory{Forgejo: stub, Err: credErr})
	st := s.GetTool(tool)
	if st == nil {
		t.Fatalf("tool %s not registered", tool)
	}
	base := map[string]any{}
	for k, v := range args {
		if v == nil {
			delete(base, k) // a nil value means "leave the argument out"
			continue
		}
		base[k] = v
	}
	res, err := st.Handler(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Name: tool, Arguments: base}})
	var out string
	switch {
	case err != nil:
		out = "ERROR: " + err.Error()
	case res.IsError:
		out = "RESULT-ERROR: " + res.Content[0].(mcp.TextContent).Text
	default:
		out = res.Content[0].(mcp.TextContent).Text
	}
	return fmt.Sprintf("%s\n-- port calls --\n%s\n", out, strings.Join(stub.calls, "\n"))
}

type goldenCase struct {
	name    string
	tool    string
	args    map[string]any
	fails   map[string]error
	credErr error
}

var errBoom = errors.New("boom")

var goldenCases = []goldenCase{
	// create_org
	{name: "create_org_full", tool: CreateOrgToolName, args: map[string]any{"name": "octo-org", "full_name": "Octo Org", "description": "d", "website": "https://o.example", "location": "here", "visibility": "private"}},
	{name: "create_org_name_only", tool: CreateOrgToolName, args: map[string]any{"name": "octo-org"}},
	{name: "create_org_empty_name", tool: CreateOrgToolName, args: map[string]any{"name": ""}},
	{name: "create_org_missing_name", tool: CreateOrgToolName, args: map[string]any{"name": nil}},
	{name: "create_org_wrong_type_name", tool: CreateOrgToolName, args: map[string]any{"name": float64(3)}},
	{name: "create_org_missing_name_no_credential", tool: CreateOrgToolName, args: map[string]any{"name": nil}, credErr: errors.New("no credential")},
	{name: "create_org_port_error", tool: CreateOrgToolName, args: map[string]any{"name": "octo-org"}, fails: map[string]error{"CreateOrg": errBoom}},

	// get_org
	{name: "get_org_ok", tool: GetOrgToolName, args: map[string]any{"org": "octo-org"}},
	{name: "get_org_missing", tool: GetOrgToolName, args: map[string]any{"org": nil}},
	{name: "get_org_port_error", tool: GetOrgToolName, args: map[string]any{"org": "octo-org"}, fails: map[string]error{"GetOrg": errBoom}},

	// list_my_orgs
	{name: "my_orgs_defaults", tool: ListMyOrgsToolName},
	{name: "my_orgs_paged", tool: ListMyOrgsToolName, args: map[string]any{"page": float64(2), "limit": float64(5)}},
	{name: "my_orgs_zero_page_is_first", tool: ListMyOrgsToolName, args: map[string]any{"page": float64(0)}},
	// The original handler took these only as JSON numbers; a string is absent.
	{name: "my_orgs_string_paging", tool: ListMyOrgsToolName, args: map[string]any{"page": "2", "limit": "5"}},
	{name: "my_orgs_fractional_paging", tool: ListMyOrgsToolName, args: map[string]any{"page": 2.9, "limit": 5.5}},
	{name: "my_orgs_port_error", tool: ListMyOrgsToolName, fails: map[string]error{"ListMyOrgs": errBoom}},

	// list_user_orgs
	{name: "user_orgs_ok", tool: ListUserOrgsToolName, args: map[string]any{"user": "octo", "page": float64(3), "limit": float64(4)}},
	{name: "user_orgs_defaults", tool: ListUserOrgsToolName, args: map[string]any{"user": "octo"}},
	{name: "user_orgs_missing_user", tool: ListUserOrgsToolName, args: map[string]any{"user": nil}},
	{name: "user_orgs_string_limit", tool: ListUserOrgsToolName, args: map[string]any{"user": "octo", "limit": "4"}},
	{name: "user_orgs_port_error", tool: ListUserOrgsToolName, args: map[string]any{"user": "octo"}, fails: map[string]error{"ListUserOrgs": errBoom}},

	// edit_org
	{name: "edit_org_full", tool: EditOrgToolName, args: map[string]any{"org": "octo-org", "full_name": "New", "description": "d", "website": "w", "location": "l", "visibility": "limited"}},
	{name: "edit_org_only_org", tool: EditOrgToolName, args: map[string]any{"org": "octo-org"}},
	{name: "edit_org_missing_org", tool: EditOrgToolName, args: map[string]any{"org": nil}},
	{name: "edit_org_edit_error", tool: EditOrgToolName, args: map[string]any{"org": "octo-org"}, fails: map[string]error{"EditOrg": errBoom}},
	{name: "edit_org_refetch_error", tool: EditOrgToolName, args: map[string]any{"org": "octo-org"}, fails: map[string]error{"GetOrg": errBoom}},

	// delete_org
	{name: "delete_org_ok", tool: DeleteOrgToolName, args: map[string]any{"org": "octo-org"}},
	{name: "delete_org_missing", tool: DeleteOrgToolName, args: map[string]any{"org": nil}},
	{name: "delete_org_port_error", tool: DeleteOrgToolName, args: map[string]any{"org": "octo-org"}, fails: map[string]error{"DeleteOrg": errBoom}},

	// list_org_members
	{name: "members_defaults", tool: ListOrgMembersToolName, args: map[string]any{"org": "octo-org"}},
	{name: "members_paged", tool: ListOrgMembersToolName, args: map[string]any{"org": "octo-org", "page": float64(2), "limit": float64(10)}},
	{name: "members_missing_org", tool: ListOrgMembersToolName, args: map[string]any{"org": nil}},
	{name: "members_string_page", tool: ListOrgMembersToolName, args: map[string]any{"org": "octo-org", "page": "2"}},
	{name: "members_port_error", tool: ListOrgMembersToolName, args: map[string]any{"org": "octo-org"}, fails: map[string]error{"ListOrgMembers": errBoom}},

	// check_org_membership
	{name: "membership_member", tool: CheckOrgMembershipToolName, args: map[string]any{"org": "octo-org", "user": "octo"}},
	{name: "membership_not_member", tool: CheckOrgMembershipToolName, args: map[string]any{"org": "octo-org", "user": "bob"}},
	{name: "membership_missing_org", tool: CheckOrgMembershipToolName, args: map[string]any{"org": nil, "user": "bob"}},
	{name: "membership_missing_user", tool: CheckOrgMembershipToolName, args: map[string]any{"org": "octo-org", "user": nil}},
	{name: "membership_port_error", tool: CheckOrgMembershipToolName, args: map[string]any{"org": "octo-org", "user": "bob"}, fails: map[string]error{"CheckOrgMembership": errBoom}},
	// A missing org is the shared not-found outcome, not "not a member".
	{name: "membership_unknown_org", tool: CheckOrgMembershipToolName, args: map[string]any{"org": "nothing", "user": "octo"}, fails: map[string]error{"CheckOrgMembership": &forge.StatusError{StatusCode: 404, Err: errors.New("organisation nothing not found")}}},

	// remove_org_member
	{name: "remove_member_ok", tool: RemoveOrgMemberToolName, args: map[string]any{"org": "octo-org", "user": "bob"}},
	{name: "remove_member_missing_org", tool: RemoveOrgMemberToolName, args: map[string]any{"org": nil, "user": "bob"}},
	{name: "remove_member_missing_user", tool: RemoveOrgMemberToolName, args: map[string]any{"org": "octo-org", "user": nil}},
	{name: "remove_member_port_error", tool: RemoveOrgMemberToolName, args: map[string]any{"org": "octo-org", "user": "bob"}, fails: map[string]error{"RemoveOrgMember": errBoom}},

	// list_org_teams
	{name: "teams_defaults", tool: ListOrgTeamsToolName, args: map[string]any{"org": "octo-org"}},
	{name: "teams_paged", tool: ListOrgTeamsToolName, args: map[string]any{"org": "octo-org", "page": float64(3), "limit": float64(7)}},
	{name: "teams_missing_org", tool: ListOrgTeamsToolName, args: map[string]any{"org": nil}},
	{name: "teams_port_error", tool: ListOrgTeamsToolName, args: map[string]any{"org": "octo-org"}, fails: map[string]error{"ListOrgTeams": errBoom}},

	// create_org_team
	{name: "create_team_full", tool: CreateOrgTeamToolName, args: map[string]any{"org": "octo-org", "name": "core", "description": "d", "permission": "admin", "can_create_org_repo": true, "includes_all_repositories": true}},
	{name: "create_team_defaults", tool: CreateOrgTeamToolName, args: map[string]any{"org": "octo-org", "name": "core"}},
	{name: "create_team_empty_permission", tool: CreateOrgTeamToolName, args: map[string]any{"org": "octo-org", "name": "core", "permission": ""}},
	{name: "create_team_wrong_type_flags", tool: CreateOrgTeamToolName, args: map[string]any{"org": "octo-org", "name": "core", "can_create_org_repo": "yes", "includes_all_repositories": float64(1)}},
	{name: "create_team_missing_org", tool: CreateOrgTeamToolName, args: map[string]any{"org": nil, "name": "core"}},
	{name: "create_team_missing_name", tool: CreateOrgTeamToolName, args: map[string]any{"org": "octo-org", "name": nil}},
	{name: "create_team_port_error", tool: CreateOrgTeamToolName, args: map[string]any{"org": "octo-org", "name": "core"}, fails: map[string]error{"CreateTeam": errBoom}},

	// add_team_member and remove_team_member
	{name: "add_member_ok", tool: AddTeamMemberToolName, args: map[string]any{"team_id": float64(5), "user": "bob"}},
	{name: "add_member_fractional_team", tool: AddTeamMemberToolName, args: map[string]any{"team_id": 5.9, "user": "bob"}},
	{name: "add_member_string_team", tool: AddTeamMemberToolName, args: map[string]any{"team_id": "5", "user": "bob"}},
	{name: "add_member_missing_team", tool: AddTeamMemberToolName, args: map[string]any{"team_id": nil, "user": "bob"}},
	{name: "add_member_missing_user", tool: AddTeamMemberToolName, args: map[string]any{"team_id": float64(5), "user": nil}},
	{name: "add_member_port_error", tool: AddTeamMemberToolName, args: map[string]any{"team_id": float64(5), "user": "bob"}, fails: map[string]error{"AddTeamMember": errBoom}},
	{name: "remove_member_team_ok", tool: RemoveTeamMemberToolName, args: map[string]any{"team_id": float64(5), "user": "bob"}},
	{name: "remove_member_team_missing_team", tool: RemoveTeamMemberToolName, args: map[string]any{"team_id": nil, "user": "bob"}},
	{name: "remove_member_team_port_error", tool: RemoveTeamMemberToolName, args: map[string]any{"team_id": float64(5), "user": "bob"}, fails: map[string]error{"RemoveTeamMember": errBoom}},

	// add_team_repo and remove_team_repo
	{name: "add_team_repo_ok", tool: AddTeamRepoToolName, args: map[string]any{"team_id": float64(5), "org": "octo-org", "repo": "hello"}},
	{name: "add_team_repo_missing_team", tool: AddTeamRepoToolName, args: map[string]any{"team_id": nil, "org": "octo-org", "repo": "hello"}},
	{name: "add_team_repo_missing_org", tool: AddTeamRepoToolName, args: map[string]any{"team_id": float64(5), "org": nil, "repo": "hello"}},
	{name: "add_team_repo_missing_repo", tool: AddTeamRepoToolName, args: map[string]any{"team_id": float64(5), "org": "octo-org", "repo": nil}},
	{name: "add_team_repo_port_error", tool: AddTeamRepoToolName, args: map[string]any{"team_id": float64(5), "org": "octo-org", "repo": "hello"}, fails: map[string]error{"AddTeamRepository": errBoom}},
	{name: "remove_team_repo_ok", tool: RemoveTeamRepoToolName, args: map[string]any{"team_id": float64(5), "org": "octo-org", "repo": "hello"}},
	{name: "remove_team_repo_missing_repo", tool: RemoveTeamRepoToolName, args: map[string]any{"team_id": float64(5), "org": "octo-org", "repo": nil}},
	{name: "remove_team_repo_port_error", tool: RemoveTeamRepoToolName, args: map[string]any{"team_id": float64(5), "org": "octo-org", "repo": "hello"}, fails: map[string]error{"RemoveTeamRepository": errBoom}},
}

func TestGolden(t *testing.T) {
	t.Parallel()
	for _, c := range goldenCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			stub := &stubOrgs{fails: c.fails}
			mcptest.Golden(t, c.name, []byte(invoke(t, stub, c.credErr, c.tool, c.args)))
		})
	}
}

// MaxResponseItems reports an unknown ceiling: the organization goldens pin the
// paging contract without a ceiling, as they always have.
func (s *stubOrgs) MaxResponseItems(context.Context) (int, bool) { return 0, false }
