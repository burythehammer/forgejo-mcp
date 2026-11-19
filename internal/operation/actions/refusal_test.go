// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge/forgetest"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge/sdk"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// TestRefusedToolsReportForgejoMessage runs each tool that writes a secret or
// dispatches a workflow through the real SDK adapter against the HTTP stub, with
// Forgejo refusing the call. The tool must report the status and Forgejo's
// message, not a bare status.
func TestRefusedToolsReportForgejoMessage(t *testing.T) {
	t.Parallel()
	repoArgs := map[string]any{"owner": testOwner, "repo": testRepo}
	orgArgs := map[string]any{"org": testOwner}
	with := func(base map[string]any, extra map[string]any) map[string]any {
		out := map[string]any{}
		for k, v := range base {
			out[k] = v
		}
		for k, v := range extra {
			out[k] = v
		}
		return out
	}
	cases := []struct {
		name    string
		tool    string
		args    map[string]any
		op      string // the port method Forgejo refuses
		status  int
		message string
	}{
		{"dispatch_workflow", DispatchWorkflowToolName,
			with(repoArgs, map[string]any{"workflow": "ci.yaml", "ref": "nope"}),
			"DispatchWorkflow", 422, "ref does not exist"},
		{"create_or_update_repo_action_secret", CreateOrUpdateRepoActionSecretToolName,
			with(repoArgs, map[string]any{"secret_name": "TOKEN", "data": "x"}),
			"SetRepoActionSecret", 403, "secrets are restricted"},
		{"delete_repo_action_secret", DeleteRepoActionSecretToolName,
			with(repoArgs, map[string]any{"secret_name": "TOKEN"}),
			"DeleteRepoActionSecret", 404, "secret does not exist"},
		{"create_or_update_org_action_secret", CreateOrUpdateOrgActionSecretToolName,
			with(orgArgs, map[string]any{"secret_name": "TOKEN", "data": "x"}),
			"SetOrgActionSecret", 422, "secret data is empty"},
		{"delete_org_action_secret", DeleteOrgActionSecretToolName,
			with(orgArgs, map[string]any{"secret_name": "TOKEN"}),
			"DeleteOrgActionSecret", 403, "not an organization owner"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			stub := forgetest.NewStub(t)
			stub.Fake.AddRepo(testOwner, testRepo)
			stub.Fake.Refuse(c.op, c.status, c.message)
			port := sdk.NewAdapter(stub.NewClient(t)).WithRawBaseURL(stub.URL)
			s := server.NewMCPServer("test", "0")
			RegisterTool(s, fake.Factory{Forgejo: port})

			res, err := s.GetTool(c.tool).Handler(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Name: c.tool, Arguments: c.args}})
			got := ""
			switch {
			case err != nil:
				got = err.Error()
			case res != nil && res.IsError:
				got = textOf(res)
			default:
				t.Fatalf("%s: want a refusal, got %+v", c.tool, res)
			}
			want := fmt.Sprintf("HTTP %d: %s", c.status, c.message)
			if !strings.Contains(got, want) {
				t.Fatalf("%s: got %q, want it to carry %q", c.tool, got, want)
			}
		})
	}
}
