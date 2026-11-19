// SPDX-License-Identifier: GPL-3.0-or-later

package user

import (
	"context"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/mcptest"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// The adapter tests are thin on purpose: behaviour is tested on the use cases
// (internal/app/user) and pinned on the wire by golden_test.go. What is left to check
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

// callTool runs one user tool through the server it is registered on, so the
// request pipeline handles the call as it does in production.
func callTool(t *testing.T, h handlers, name string, args map[string]any) (*mcp.CallToolResult, error) {
	t.Helper()
	s := server.NewMCPServer("test", "0")
	RegisterTool(s, h.forge)
	return s.GetTool(name).Handler(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Name: name, Arguments: args}})
}

func TestToolsReachTheirUseCases(t *testing.T) {
	t.Parallel()
	h, _ := newFixture()

	res, err := callTool(t, h, GetMyUserInfoToolName, nil)
	if err != nil || res == nil || res.IsError {
		t.Fatalf("my info: err=%v res=%+v", err, res)
	}
	if text := textOf(res); !strings.Contains(text, `"login"`) {
		t.Fatalf("my info is not a user: %s", text)
	}

	res, err = callTool(t, h, CheckNotificationsToolName, map[string]any{"since": "2026-01-01T00:00:00Z"})
	if err != nil || res == nil || res.IsError {
		t.Fatalf("check: err=%v res=%+v", err, res)
	}

	res, err = callTool(t, h, MarkAllNotificationsReadToolName, nil)
	if err != nil || res == nil || res.IsError || !strings.Contains(textOf(res), "All notifications marked as read") {
		t.Fatalf("mark all: err=%v res=%+v", err, res)
	}

	res, err = callTool(t, h, GetNotificationThreadToolName, nil)
	if err != nil || res == nil || !res.IsError || textOf(res) != "id is required and must be a number" {
		t.Fatalf("thread without id: err=%v res=%+v", err, res)
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
