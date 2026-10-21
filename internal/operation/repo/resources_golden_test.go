// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"
	"strings"
	"testing"
	"time"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcptest"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/ptr"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

// TestRepoReadGolden_ToolHasTopicsResourceDoesNot pins an intentional
// difference: get_repo carries the topics, the forgejo://repo resource does not.
// Both are pinned so a change to either shows up here.
func TestRepoReadGolden_ToolHasTopicsResourceDoesNot(t *testing.T) {
	t.Parallel()
	h, f := adminFixture(t)
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	f.Now = func() time.Time { return at }
	_, _ = f.EditRepo(context.Background(), adminOwner, adminRepoName, forgejo_sdk.EditRepoOption{Description: ptr.To("MCP server")})
	_ = f.SetRepoTopics(context.Background(), adminOwner, adminRepoName, []string{"go", "mcp"})

	tool := adminOK(t, h.GetRepoFn, nil)
	mcptest.Golden(t, "tool_get_repo_with_topics", []byte(tool))

	contents, err := h.repoResourceHandler(context.Background(), repoResourceRequest(adminOwner, adminRepoName))
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, c := range contents {
		b.WriteString(c.(mcp.TextResourceContents).Text)
	}
	mcptest.Golden(t, "resource_repo_without_topics", []byte(b.String()))
}
