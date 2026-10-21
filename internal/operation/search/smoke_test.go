// SPDX-License-Identifier: GPL-3.0-or-later

package search

import (
	"context"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/forgetest"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/sdk"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// TestSmokeOverHTTP runs the search tools through the real SDK adapter against
// a stub Forgejo over HTTP, end to end.
func TestSmokeOverHTTP(t *testing.T) {
	t.Parallel()
	stub := forgetest.NewStub(t)
	stub.Fake.SeedUser(forgejo_sdk.User{UserName: "alice"})
	stub.Fake.SeedTeam("acme", forgejo_sdk.Team{Name: "devs"})
	stub.Fake.SeedSearchableRepo(forgejo_sdk.Repository{ID: 1, Name: "tool-a"})
	port := sdk.NewAdapter(stub.NewClient(t))

	s := server.NewMCPServer("test", "0")
	RegisterTool(s, fake.Factory{Forgejo: port})
	call := func(tool string, args map[string]any) string {
		t.Helper()
		res, err := s.GetTool(tool).Handler(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: args}})
		if err != nil || res.IsError {
			t.Fatalf("%s: %v %+v", tool, err, res)
		}
		return res.Content[0].(mcp.TextContent).Text
	}

	if got := call(SearchUsersToolName, map[string]any{"keyword": "ali"}); !strings.Contains(got, "alice") {
		t.Fatalf("users: %s", got)
	}
	if got := call(SearchOrgTeamsToolName, map[string]any{"org": "acme"}); !strings.Contains(got, "devs") {
		t.Fatalf("teams: %s", got)
	}
	if got := call(SearchReposToolName, map[string]any{"keyword": "tool"}); !strings.Contains(got, "tool-a") {
		t.Fatalf("repos: %s", got)
	}
}
