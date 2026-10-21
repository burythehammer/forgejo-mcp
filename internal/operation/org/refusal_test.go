// SPDX-License-Identifier: GPL-3.0-or-later

package org

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/forgetest"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/sdk"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// TestRefusedMembershipCheckReportsForgejoMessage runs check_org_membership through
// the real SDK adapter against the HTTP stub, with Forgejo refusing the check. A
// 404 is "not a member" and stays a plain answer; any other refusal must carry the
// status and Forgejo's message.
func TestRefusedMembershipCheckReportsForgejoMessage(t *testing.T) {
	t.Parallel()
	stub := forgetest.NewStub(t)
	stub.Fake.Refuse("CheckOrgMembership", 403, "members are hidden from you")
	port := sdk.NewAdapter(stub.NewClient(t)).WithRawBaseURL(stub.URL)
	s := server.NewMCPServer("test", "0")
	RegisterTool(s, fake.Factory{Forgejo: port})

	args := map[string]any{"org": "octo", "user": "cat"}
	res, err := s.GetTool(CheckOrgMembershipToolName).Handler(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Name: CheckOrgMembershipToolName, Arguments: args}})
	got := ""
	switch {
	case err != nil:
		got = err.Error()
	case res != nil && res.IsError:
		got = fmt.Sprint(res.Content)
	default:
		t.Fatalf("want a refusal, got %+v", res)
	}
	want := "HTTP 403: members are hidden from you"
	if !strings.Contains(got, want) {
		t.Fatalf("got %q, want it to carry %q", got, want)
	}
}
