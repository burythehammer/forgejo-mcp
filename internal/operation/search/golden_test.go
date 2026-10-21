// SPDX-License-Identifier: GPL-3.0-or-later

package search

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

// These tests pin the wire contract of every search tool: its tool definition
// (schema snapshot) and its output and error text (golden), together with the
// calls the tool made on the Forgejo port. They run against a stub port that
// returns fixed, fully populated data, so the bytes are deterministic.

var allTools = []mcp.Tool{SearchUsersTool, SearchOrgTeamsTool, SearchReposTool}

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
	org  = &forgejo_sdk.Organization{ID: 6, UserName: "octo-org", FullName: "Octo Org", Visibility: "public"}
)

func fixedRepo(name string) *forgejo_sdk.Repository {
	return &forgejo_sdk.Repository{ID: 9, Name: name, FullName: "octo/" + name, Owner: user, Created: t0, Updated: t1, Description: "a repo"}
}

func fixedTeam(name string) *forgejo_sdk.Team {
	return &forgejo_sdk.Team{ID: 5, Name: name, Organization: org, Permission: forgejo_sdk.AccessModeWrite, CanCreateOrgRepo: true}
}

// stubSearch is a forge.Forgejo whose search domain returns fixed data and
// records the calls it receives. Any other domain would panic on the nil embed.
type stubSearch struct {
	forge.Forgejo
	mu    sync.Mutex
	calls []string
	fail  error
}

func (s *stubSearch) rec(name string, args ...any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, _ := json.Marshal(args)
	s.calls = append(s.calls, name+" "+string(b))
	return s.fail
}

func (s *stubSearch) SearchUsers(_ context.Context, opt forgejo_sdk.SearchUsersOption) ([]*forgejo_sdk.User, error) {
	return []*forgejo_sdk.User{user, {ID: 2, UserName: "bob", FullName: "Bob", IsAdmin: true}}, s.rec("SearchUsers", opt)
}

func (s *stubSearch) SearchOrgTeams(_ context.Context, o string, opt forgejo_sdk.SearchTeamsOptions) ([]*forgejo_sdk.Team, error) {
	return []*forgejo_sdk.Team{fixedTeam("core")}, s.rec("SearchOrgTeams", o, opt)
}

func (s *stubSearch) SearchRepos(_ context.Context, opt forgejo_sdk.SearchRepoOptions) ([]*forgejo_sdk.Repository, error) {
	return []*forgejo_sdk.Repository{fixedRepo("hello"), fixedRepo("world")}, s.rec("SearchRepos", opt)
}

// invoke calls a registered tool the way the server does and renders what the
// client would see: the text, or the error, then the port calls made.
func invoke(t *testing.T, stub *stubSearch, tool string, args map[string]any) string {
	t.Helper()
	s := server.NewMCPServer("test", "0")
	RegisterTool(s, fake.Factory{Forgejo: stub})
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
	name string
	tool string
	args map[string]any
	stub func(*stubSearch)
}

var goldenCases = []goldenCase{
	{name: "users_defaults", tool: SearchUsersToolName},
	{name: "users_keyword", tool: SearchUsersToolName, args: map[string]any{"keyword": "oct", "page": float64(2), "limit": float64(5)}},
	// The original handlers read numbers forgivingly: a fractional number is
	// truncated, a string number is parsed, and a malformed one falls back to
	// the default.
	{name: "users_fractional_paging", tool: SearchUsersToolName, args: map[string]any{"page": 2.9, "limit": 3.5}},
	{name: "users_string_paging", tool: SearchUsersToolName, args: map[string]any{"page": "7", "limit": "9"}},
	{name: "users_malformed_paging", tool: SearchUsersToolName, args: map[string]any{"page": "soon", "limit": true}},
	{name: "users_wrong_keyword_type", tool: SearchUsersToolName, args: map[string]any{"keyword": float64(4)}},
	{name: "users_port_error", tool: SearchUsersToolName, args: map[string]any{"keyword": "x"}, stub: func(s *stubSearch) { s.fail = errors.New("boom") }},

	{name: "org_teams_missing_org", tool: SearchOrgTeamsToolName, args: map[string]any{"org": nil}},
	{name: "org_teams_wrong_org_type", tool: SearchOrgTeamsToolName, args: map[string]any{"org": float64(6)}},
	{name: "org_teams_defaults", tool: SearchOrgTeamsToolName, args: map[string]any{"org": "octo-org"}},
	{name: "org_teams_keyword", tool: SearchOrgTeamsToolName, args: map[string]any{"org": "octo-org", "keyword": "co", "page": float64(3), "limit": float64(4)}},
	{name: "org_teams_fractional_paging", tool: SearchOrgTeamsToolName, args: map[string]any{"org": "octo-org", "page": 1.5, "limit": 2.5}},
	{name: "org_teams_port_error", tool: SearchOrgTeamsToolName, args: map[string]any{"org": "octo-org"}, stub: func(s *stubSearch) { s.fail = errors.New("boom") }},

	{name: "repos_defaults", tool: SearchReposToolName},
	{name: "repos_filtered", tool: SearchReposToolName, args: map[string]any{"keyword": "hel", "sort": "stars", "order": "asc", "page": float64(2), "limit": float64(10)}},
	{name: "repos_string_paging", tool: SearchReposToolName, args: map[string]any{"page": "2", "limit": "3"}},
	{name: "repos_port_error", tool: SearchReposToolName, args: map[string]any{"keyword": "x"}, stub: func(s *stubSearch) { s.fail = errors.New("boom") }},
}

func TestGolden(t *testing.T) {
	t.Parallel()
	for _, c := range goldenCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			stub := &stubSearch{}
			if c.stub != nil {
				c.stub(stub)
			}
			mcptest.Golden(t, c.name, []byte(invoke(t, stub, c.tool, c.args)))
		})
	}
}

// MaxResponseItems reports an unknown ceiling: the search goldens pin the
// paging contract without a ceiling, as they always have.
func (s *stubSearch) MaxResponseItems(context.Context) (int, bool) { return 0, false }
