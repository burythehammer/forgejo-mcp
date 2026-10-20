// SPDX-License-Identifier: GPL-3.0-or-later

package org

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

// TestSmokeOverHTTP runs the organization, membership and team tools through
// the real SDK adapter against a stub Forgejo over HTTP, end to end.
func TestSmokeOverHTTP(t *testing.T) {
	t.Parallel()
	stub := forgetest.NewStub(t)
	port := sdk.NewAdapter(stub.NewClient(t)).WithRawBaseURL(stub.URL)

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

	if got := call(CreateOrgToolName, map[string]any{"name": "octo-org"}); !strings.Contains(got, "octo-org") {
		t.Fatalf("create: %s", got)
	}
	if got := call(EditOrgToolName, map[string]any{"org": "octo-org", "full_name": "Octo Renamed"}); !strings.Contains(got, "Octo Renamed") {
		t.Fatalf("edit: %s", got)
	}
	if got := call(CreateOrgTeamToolName, map[string]any{"org": "octo-org", "name": "core"}); !strings.Contains(got, `"permission":"read"`) {
		t.Fatalf("team: %s", got)
	}
	if got := call(CheckOrgMembershipToolName, map[string]any{"org": "octo-org", "user": "nobody"}); !strings.Contains(got, `"is_member":false`) {
		t.Fatalf("membership: %s", got)
	}
	if got := call(DeleteOrgToolName, map[string]any{"org": "octo-org"}); !strings.Contains(got, "deleted successfully") {
		t.Fatalf("delete: %s", got)
	}
}
