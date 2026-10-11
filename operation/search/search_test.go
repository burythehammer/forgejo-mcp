// SPDX-License-Identifier: GPL-3.0-or-later

package search

import (
	"context"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/fake"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

func newFixture() (handlers, *fake.Forgejo) {
	f := fake.New()
	return handlers{forge: fake.Factory{Forgejo: f}}, f
}

func run(t *testing.T, fn func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error), args map[string]any) string {
	t.Helper()
	res, err := fn(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: args}})
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

func TestSearchUserFn_MatchesKeyword(t *testing.T) {
	t.Parallel()
	h, f := newFixture()
	f.SeedUser(forgejo_sdk.User{UserName: "alice"})
	f.SeedUser(forgejo_sdk.User{UserName: "bob"})

	out := run(t, h.SearchUserFn, map[string]any{"keyword": "ali"})

	if !strings.Contains(out, "alice") || strings.Contains(out, "bob") {
		t.Errorf("unexpected result: %s", out)
	}
}

func TestSearchUserFn_Pagination(t *testing.T) {
	t.Parallel()
	h, f := newFixture()
	for _, n := range []string{"u1", "u2", "u3"} {
		f.SeedUser(forgejo_sdk.User{UserName: n})
	}

	out := run(t, h.SearchUserFn, map[string]any{"page": 2.0, "limit": 2.0})

	if !strings.Contains(out, "u3") || strings.Contains(out, "u1") {
		t.Errorf("unexpected result: %s", out)
	}
}

func TestSearchOrgTeamsFn(t *testing.T) {
	t.Parallel()
	h, f := newFixture()
	f.SeedTeam("acme", forgejo_sdk.Team{Name: "devs"})
	f.SeedTeam("acme", forgejo_sdk.Team{Name: "owners"})

	out := run(t, h.SearchOrgTeamsFn, map[string]any{"org": "acme", "keyword": "dev"})

	if !strings.Contains(out, "devs") || strings.Contains(out, "owners") {
		t.Errorf("unexpected result: %s", out)
	}
}

func TestSearchOrgTeamsFn_OrgRequiredAndKnown(t *testing.T) {
	t.Parallel()
	h, _ := newFixture()

	if _, err := h.SearchOrgTeamsFn(context.Background(), mcp.CallToolRequest{}); err == nil {
		t.Error("missing org must be an error")
	}
	_, err := h.SearchOrgTeamsFn(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"org": "ghost"}}})
	if err == nil || !strings.Contains(err.Error(), "search org teams err") {
		t.Errorf("unknown org: %v", err)
	}
}

func TestSearchReposFn_KeywordAndSort(t *testing.T) {
	t.Parallel()
	h, f := newFixture()
	f.SeedSearchableRepo(forgejo_sdk.Repository{ID: 1, Name: "tool-b"})
	f.SeedSearchableRepo(forgejo_sdk.Repository{ID: 2, Name: "tool-a"})
	f.SeedSearchableRepo(forgejo_sdk.Repository{ID: 3, Name: "other"})

	out := run(t, h.SearchReposFn, map[string]any{"keyword": "tool", "sort": "alpha", "order": "desc"})

	if strings.Contains(out, "other") || strings.Index(out, "tool-b") > strings.Index(out, "tool-a") {
		t.Errorf("unexpected result: %s", out)
	}
}
