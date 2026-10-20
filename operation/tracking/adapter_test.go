// SPDX-License-Identifier: GPL-3.0-or-later

package tracking

import (
	"context"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/operation/mcptest"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge/fake"

	"github.com/mark3labs/mcp-go/mcp"
)

// The adapter tests are thin on purpose: behaviour is tested on the use cases
// (app/tracking) and pinned on the wire by golden_test.go. What is left to check
// here is that each tool's argument struct agrees with its schema, and that the
// tools reach their use cases through the real handlers and the fake.

func TestArgsMatchSchemas(t *testing.T) {
	t.Parallel()
	cases := []struct {
		tool mcp.Tool
		args any
	}{
		{ListIssueTrackedTimesTool, issueTimesArgs{}},
		{ListRepoTrackedTimesTool, repoTimesArgs{}},
		{ListMyTrackedTimesTool, noArgs{}},
		{AddIssueTimeTool, addTimeArgs{}},
		{ResetIssueTimeTool, issueArgs{}},
		{DeleteIssueTimeEntryTool, deleteEntryArgs{}},
		{StartIssueStopwatchTool, issueArgs{}},
		{StopIssueStopwatchTool, issueArgs{}},
		{CancelIssueStopwatchTool, issueArgs{}},
		{ListMyStopwatchesTool, noArgs{}},
	}
	for _, c := range cases {
		t.Run(c.tool.Name, func(t *testing.T) { mcptest.ArgsMatchSchema(t, c.tool, c.args) })
	}
}

func TestToolsReachTheirUseCases(t *testing.T) {
	t.Parallel()
	f := fake.New()
	f.AddTrackableIssue("octo", "hello", 1, "work")
	h := handlers{forge: fake.Factory{Forgejo: f}}
	base := map[string]any{"owner": "octo", "repo": "hello", "index": float64(1)}
	with := func(extra map[string]any) mcp.CallToolRequest {
		args := map[string]any{}
		for k, v := range base {
			args[k] = v
		}
		for k, v := range extra {
			args[k] = v
		}
		return mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: args}}
	}
	ctx := context.Background()

	res, err := h.AddIssueTimeFn(ctx, with(map[string]any{"duration": "5m"}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("add: err=%v res=%+v", err, res)
	}
	if text := textOf(res); !strings.Contains(text, `"time":300`) {
		t.Fatalf("add logged the wrong time: %s", text)
	}

	res, err = h.ListIssueTrackedTimesFn(ctx, with(nil))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("list: err=%v res=%+v", err, res)
	}
	if text := textOf(res); !strings.Contains(text, `"time":300`) {
		t.Fatalf("list did not return the entry: %s", text)
	}

	if _, err := h.StopIssueStopwatchFn(ctx, with(nil)); err == nil || !strings.Contains(err.Error(), "stop issue stopwatch err") {
		t.Fatalf("stop without start: %v", err)
	}
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
