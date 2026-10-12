// SPDX-License-Identifier: GPL-3.0-or-later

package user

import (
	"context"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/mcptest"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/fake"

	"github.com/mark3labs/mcp-go/mcp"
)

// The adapter tests are thin on purpose: behaviour is tested on the use cases
// (app/user) and pinned on the wire by golden_test.go. What is left to check
// here is that each tool's argument struct agrees with its schema, and that the
// tools reach their use cases through the real handlers and the fake.

func TestArgsMatchSchemas(t *testing.T) {
	t.Parallel()
	cases := []struct {
		tool mcp.Tool
		args any
	}{
		{GetMyUserInfoTool, noArgs{}},
		{CheckNotificationsTool, checkArgs{}},
		{GetNotificationThreadTool, threadArgs{}},
		{MarkNotificationReadTool, threadArgs{}},
		{MarkAllNotificationsReadTool, markAllArgs{}},
		{ListRepoNotificationsTool, repoCheckArgs{}},
		{MarkRepoNotificationsReadTool, repoMarkArgs{}},
	}
	for _, c := range cases {
		t.Run(c.tool.Name, func(t *testing.T) { mcptest.ArgsMatchSchema(t, c.tool, c.args) })
	}
}

type noArgs struct{}

// newFixture returns handlers served by a fresh in-memory fake. The owner
// resource tests share it.
func newFixture() (handlers, *fake.Forgejo) {
	f := fake.New()
	return handlers{forge: fake.Factory{Forgejo: f}}, f
}

func TestToolsReachTheirUseCases(t *testing.T) {
	t.Parallel()
	h, _ := newFixture()
	ctx := context.Background()
	req := func(args map[string]any) mcp.CallToolRequest {
		return mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: args}}
	}

	res, err := h.GetUserInfoFn(ctx, req(nil))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("my info: err=%v res=%+v", err, res)
	}
	if text := textOf(res); !strings.Contains(text, `"login"`) {
		t.Fatalf("my info is not a user: %s", text)
	}

	res, err = h.CheckNotificationsFn(ctx, req(map[string]any{"since": "2026-01-01T00:00:00Z"}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("check: err=%v res=%+v", err, res)
	}

	res, err = h.MarkAllNotificationsReadFn(ctx, req(nil))
	if err != nil || res == nil || res.IsError || !strings.Contains(textOf(res), "All notifications marked as read") {
		t.Fatalf("mark all: err=%v res=%+v", err, res)
	}

	if _, err := h.GetNotificationThreadFn(ctx, req(nil)); err == nil || err.Error() != "id is required and must be a number" {
		t.Fatalf("thread without id: %v", err)
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
