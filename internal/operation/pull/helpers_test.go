// SPDX-License-Identifier: GPL-3.0-or-later

package pull

import (
	"context"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

// Handler tests run against the in-memory fake, build their own, and so run in
// parallel with no shared state.

const (
	testOwner = "octo"
	testRepo  = "hello"
)

// newFixture returns handlers served by a fresh fake that already has the
// repository testOwner/testRepo.
func newFixture(t *testing.T) (handlers, *fake.Forgejo) {
	t.Helper()
	f := fake.New()
	f.AddRepo(testOwner, testRepo)
	return handlers{forge: fake.Factory{Forgejo: f}}, f
}

// openPR creates a pull request through the fake's port and returns its index.
func openPR(t *testing.T, f *fake.Forgejo, title string) int64 {
	t.Helper()
	pr, err := f.CreatePullRequest(context.Background(), testOwner, testRepo, forgejo_sdk.CreatePullRequestOption{
		Head: "feature", Base: "main", Title: title, Body: "body of " + title,
	})
	if err != nil {
		t.Fatalf("seeding pull request: %v", err)
	}
	return pr.Index
}

func makeReq(args map[string]any) mcp.CallToolRequest {
	base := map[string]any{"owner": testOwner, "repo": testRepo}
	for k, v := range args {
		base[k] = v
	}
	return mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: base}}
}

func textOf(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

// call runs a tool handler and fails the test unless it succeeded.
func call(t *testing.T, fn func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error), args map[string]any) string {
	t.Helper()
	res, err := fn(context.Background(), makeReq(args))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("err=%v res=%+v", err, res)
	}
	return textOf(res)
}
