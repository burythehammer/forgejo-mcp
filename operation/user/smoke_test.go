// SPDX-License-Identifier: GPL-3.0-or-later

package user

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

// TestSmokeOverHTTP runs the user and notification tools through the real SDK
// adapter against a stub Forgejo over HTTP, end to end.
func TestSmokeOverHTTP(t *testing.T) {
	t.Parallel()
	stub := forgetest.NewStub(t)
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

	if got := call(GetMyUserInfoToolName, nil); !strings.Contains(got, `"login"`) {
		t.Fatalf("my info: %s", got)
	}
	if got := call(CheckNotificationsToolName, map[string]any{"since": "2026-01-01T00:00:00Z"}); !strings.Contains(got, "Result") {
		t.Fatalf("notifications: %s", got)
	}
	if got := call(MarkAllNotificationsReadToolName, nil); !strings.Contains(got, "All notifications marked as read") {
		t.Fatalf("mark all: %s", got)
	}
}
