// SPDX-License-Identifier: GPL-3.0-or-later

package search

import (
	"context"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcptest"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

// The adapter tests are thin on purpose: behaviour is tested on the use cases
// (internal/app/search) and pinned on the wire by golden_test.go. What is left to check
// here is that each tool's argument struct agrees with its schema, and that a
// tool reaches its use case through the real handler.

func TestArgsMatchSchemas(t *testing.T) {
	t.Parallel()
	cases := []struct {
		tool mcp.Tool
		args any
	}{
		{SearchUsersTool, usersArgs{}},
		{SearchOrgTeamsTool, orgTeamsArgs{}},
		{SearchReposTool, reposArgs{}},
	}
	for _, c := range cases {
		t.Run(c.tool.Name, func(t *testing.T) { mcptest.ArgsMatchSchema(t, c.tool, c.args) })
	}
}

func TestToolsReachTheirUseCases(t *testing.T) {
	t.Parallel()
	f := fake.New()
	f.SeedUser(forgejo_sdk.User{UserName: "alice"})
	f.SeedTeam("acme", forgejo_sdk.Team{Name: "devs"})
	f.SeedSearchableRepo(forgejo_sdk.Repository{ID: 1, Name: "tool-a"})
	h := handlers{forge: fake.Factory{Forgejo: f}}

	cases := []struct {
		name string
		fn   func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
		args map[string]any
		want string
	}{
		{"users", h.SearchUserFn, map[string]any{"keyword": "ali"}, "alice"},
		{"org teams", h.SearchOrgTeamsFn, map[string]any{"org": "acme"}, "devs"},
		{"repos", h.SearchReposFn, map[string]any{"keyword": "tool"}, "tool-a"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			res, err := c.fn(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: c.args}})
			if err != nil || res == nil || res.IsError {
				t.Fatalf("err=%v res=%+v", err, res)
			}
			text := res.Content[0].(mcp.TextContent).Text
			if !strings.Contains(text, c.want) {
				t.Fatalf("want %q in %s", c.want, text)
			}
		})
	}
}
