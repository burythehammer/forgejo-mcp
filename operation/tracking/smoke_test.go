// SPDX-License-Identifier: GPL-3.0-or-later

package tracking

import (
	"context"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/fake"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/forgetest"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/sdk"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// TestSmokeOverHTTP runs the time-tracking tools through the real SDK adapter
// against a stub Forgejo over HTTP, end to end.
func TestSmokeOverHTTP(t *testing.T) {
	t.Parallel()
	stub := forgetest.NewStub(t)
	stub.Fake.AddTrackableIssue("octo", "hello", 1, "work")
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
	issue := map[string]any{"owner": "octo", "repo": "hello", "index": float64(1)}
	with := func(extra map[string]any) map[string]any {
		out := map[string]any{}
		for k, v := range issue {
			out[k] = v
		}
		for k, v := range extra {
			out[k] = v
		}
		return out
	}

	if got := call(AddIssueTimeToolName, with(map[string]any{"duration": "5m"})); !strings.Contains(got, `"time":300`) {
		t.Fatalf("add: %s", got)
	}
	if got := call(ListIssueTrackedTimesToolName, with(nil)); !strings.Contains(got, `"time":300`) {
		t.Fatalf("list: %s", got)
	}
	if got := call(StartIssueStopwatchToolName, with(nil)); !strings.Contains(got, "Stopwatch started") {
		t.Fatalf("start: %s", got)
	}
	if got := call(CancelIssueStopwatchToolName, with(nil)); !strings.Contains(got, "Stopwatch cancelled") {
		t.Fatalf("cancel: %s", got)
	}
}
