// SPDX-License-Identifier: GPL-3.0-or-later

package org

import (
	"context"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/operation/mcptest"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge/fake"

	"github.com/mark3labs/mcp-go/mcp"
)

// The adapter tests are thin on purpose: behaviour is tested on the use cases
// (app/org) and pinned on the wire by golden_test.go. What is left to check
// here is that each tool's argument struct agrees with its schema, and that the
// tools reach their use cases through the real handlers and the fake.

func TestArgsMatchSchemas(t *testing.T) {
	t.Parallel()
	cases := []struct {
		tool mcp.Tool
		args any
	}{
		{CreateOrgTool, createOrgArgs{}},
		{GetOrgTool, orgArgs{}},
		{ListMyOrgsTool, myOrgsArgs{}},
		{ListUserOrgsTool, userOrgsArgs{}},
		{EditOrgTool, editOrgArgs{}},
		{DeleteOrgTool, orgArgs{}},
		{ListOrgMembersTool, membersArgs{}},
		{CheckOrgMembershipTool, membershipArgs{}},
		{RemoveOrgMemberTool, membershipArgs{}},
		{ListOrgTeamsTool, teamsArgs{}},
		{CreateOrgTeamTool, createTeamArgs{}},
		{AddTeamMemberTool, teamMemberArgs{}},
		{RemoveTeamMemberTool, teamMemberArgs{}},
		{AddTeamRepoTool, teamRepoArgs{}},
		{RemoveTeamRepoTool, teamRepoArgs{}},
	}
	for _, c := range cases {
		t.Run(c.tool.Name, func(t *testing.T) { mcptest.ArgsMatchSchema(t, c.tool, c.args) })
	}
}

func TestToolsReachTheirUseCases(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := fake.New()
	h := handlers{forge: fake.Factory{Forgejo: f}}
	req := func(args map[string]any) mcp.CallToolRequest {
		return mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: args}}
	}
	mustText := func(name string, res *mcp.CallToolResult, err error) string {
		t.Helper()
		if err != nil || res == nil || res.IsError {
			t.Fatalf("%s: err=%v res=%+v", name, err, res)
		}
		var b strings.Builder
		for _, c := range res.Content {
			if tc, ok := c.(mcp.TextContent); ok {
				b.WriteString(tc.Text)
			}
		}
		return b.String()
	}

	res, err := h.CreateOrgFn(ctx, req(map[string]any{"name": "octo-org"}))
	if text := mustText("create", res, err); !strings.Contains(text, "octo-org") {
		t.Fatalf("create: %s", text)
	}

	res, err = h.EditOrgFn(ctx, req(map[string]any{"org": "octo-org", "full_name": "Octo Org Renamed"}))
	if text := mustText("edit", res, err); !strings.Contains(text, "Octo Org Renamed") {
		t.Fatalf("edit did not return the updated org: %s", text)
	}

	res, err = h.CreateOrgTeamFn(ctx, req(map[string]any{"org": "octo-org", "name": "core"}))
	if text := mustText("create team", res, err); !strings.Contains(text, `"permission":"read"`) {
		t.Fatalf("team did not default to read: %s", text)
	}

	res, err = h.CheckOrgMembershipFn(ctx, req(map[string]any{"org": "octo-org", "user": "nobody"}))
	if text := mustText("membership", res, err); !strings.Contains(text, `"is_member":false`) {
		t.Fatalf("membership: %s", text)
	}

	if _, err := h.GetOrgFn(ctx, req(map[string]any{"org": "ghost"})); err == nil || !strings.HasPrefix(err.Error(), "get org err: ") {
		t.Fatalf("unknown org: %v", err)
	}
}
