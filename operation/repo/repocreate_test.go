// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/fake"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestCreateForkAndListMyRepos(t *testing.T) {
	t.Parallel()
	h, f := adminFixture(t)
	call := func(fn toolFn, args map[string]any) *mcp.CallToolResult {
		t.Helper()
		res, err := fn(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: args}})
		if err != nil || res == nil {
			t.Fatalf("err=%v res=%+v", err, res)
		}
		return res
	}
	if res := call(h.CreateRepoFn, map[string]any{"name": "fresh"}); res.IsError || !strings.Contains(adminText(res), "fresh") {
		t.Fatalf("create: %+v", res)
	}
	if res := call(h.ForkRepoFn, map[string]any{"user": adminOwner, "repo": adminRepoName}); res.IsError || !strings.Contains(adminText(res), "Fork success") {
		t.Fatalf("fork: %+v", res)
	}
	res := call(h.ListMyReposFn, map[string]any{})
	if out := adminText(res); res.IsError || !strings.Contains(out, "fresh") || !strings.Contains(out, adminRepoName) {
		t.Fatalf("list: %+v", res)
	}
	if _, err := f.GetRepo(context.Background(), fake.Poster, "fresh"); err != nil {
		t.Fatal(err)
	}
	if res := call(h.CreateRepoFn, map[string]any{"name": "fresh"}); !res.IsError {
		t.Fatal("duplicate create must be an error result")
	}
}
