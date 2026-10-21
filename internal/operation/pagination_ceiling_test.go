// SPDX-License-Identifier: GPL-3.0-or-later

package operation

import (
	"context"
	"strings"
	"testing"
	"time"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/attachment"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/pull"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/repo"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/tracking"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ceilingItems is the instance ceiling the list tools are run under.
const ceilingItems = 2

// unpagedListTools are list tools whose input schema has no page or limit: their
// goldens are the same at every call shape, so there is no client-controlled
// bound for the ceiling to apply to. Giving them one changes a tool schema,
// which this ticket does not do, so they are left as they are.
var unpagedListTools = map[string]bool{
	attachment.ListIssueAttachmentsToolName:   true,
	attachment.ListCommentAttachmentsToolName: true,
	repo.ListMyReposToolName:                  true,
	tracking.ListMyStopwatchesToolName:        true,
	tracking.ListMyTrackedTimesToolName:       true,
	pull.ListPullReviewCommentsToolName:       true,
	repo.ListRepoContentsToolName:             true,
}

// TestCeilingListTools pins the instance ceiling on every paged list tool: a
// limit above max_response_items is refused and names the ceiling, and a
// default call still succeeds because the default is lowered to the ceiling
// rather than refused.
func TestCeilingListTools(t *testing.T) {
	t.Parallel()
	for _, pt := range pageToolsTable {
		if unpagedListTools[pt.tool] {
			continue
		}
		t.Run(pt.tool+"/over_ceiling", func(t *testing.T) {
			t.Parallel()
			out := ceilingInvoke(t, pt, map[string]any{"limit": float64(ceilingItems + 1)})
			isErr := strings.HasPrefix(out, "ERROR:") || strings.HasPrefix(out, "RESULT-ERROR:")
			if !isErr || !strings.Contains(out, "max_response_items") {
				t.Fatalf("limit above ceiling was not refused with the ceiling named:\n%s", out)
			}
		})
		t.Run(pt.tool+"/default_under_ceiling", func(t *testing.T) {
			t.Parallel()
			out := ceilingInvoke(t, pt, nil)
			if strings.HasPrefix(out, "ERROR:") || strings.HasPrefix(out, "RESULT-ERROR:") {
				t.Fatalf("default call under a ceiling of %d failed:\n%s", ceilingItems, out)
			}
		})
	}
}

// ceilingInvoke is pageInvoke with the fake's ceiling set to ceilingItems.
func ceilingInvoke(t *testing.T, pt pageTool, extra map[string]any) string {
	t.Helper()
	f := fake.New()
	f.Now = func() time.Time { return pageNow }
	pt.seed(t, f)
	f.SetMaxResponseItems(ceilingItems)
	s := server.NewMCPServer("test", "0")
	RegisterTool(s, "test", fake.Factory{Forgejo: f})
	st := s.GetTool(pt.tool)
	if st == nil {
		t.Fatalf("tool %s not registered", pt.tool)
	}
	args := map[string]any{}
	for k, v := range pt.args {
		args[k] = v
	}
	for k, v := range extra {
		args[k] = v
	}
	res, err := st.Handler(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Name: pt.tool, Arguments: args}})
	switch {
	case err != nil:
		return "ERROR: " + err.Error() + "\n"
	case res.IsError:
		return "RESULT-ERROR: " + res.Content[0].(mcp.TextContent).Text + "\n"
	default:
		return res.Content[0].(mcp.TextContent).Text + "\n"
	}
}
