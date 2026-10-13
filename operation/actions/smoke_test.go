// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/fake"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/forgetest"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/sdk"

	"github.com/mark3labs/mcp-go/server"
)

// TestSmokeOverHTTP runs the actions tools through the real SDK adapter against
// the HTTP stub, so the request paths, decoding and error mapping are exercised
// end to end. The use cases and goldens cover the rest.
func TestSmokeOverHTTP(t *testing.T) {
	t.Parallel()
	stub := forgetest.NewStub(t)
	stub.Fake.AddRepo(testOwner, testRepo)
	port := sdk.NewAdapter(stub.NewClient(t))

	s := server.NewMCPServer("test", "0")
	RegisterTool(s, fake.Factory{Forgejo: port})
	call := func(tool string, args map[string]any) string {
		t.Helper()
		res, err := s.GetTool(tool).Handler(context.Background(), newCallToolRequest(repoArgs(args)))
		if err != nil || res.IsError {
			t.Fatalf("%s: %v %+v", tool, err, res)
		}
		return textOf(res)
	}

	if got := call(ListWorkflowRunsToolName, nil); !strings.Contains(got, `"workflow_runs":[]`) {
		t.Fatalf("list runs: %s", got)
	}
	if got := call(ListRepoActionSecretsToolName, nil); !strings.Contains(got, `"secrets":[]`) {
		t.Fatalf("list secrets: %s", got)
	}
	if got := call(CreateOrUpdateRepoActionSecretToolName, map[string]any{"secret_name": "TOKEN", "data": "v"}); !strings.Contains(got, "created_or_updated") {
		t.Fatalf("set secret: %s", got)
	}

	// A run the stub does not know is an error, not an empty run.
	res, err := s.GetTool(GetWorkflowRunToolName).Handler(context.Background(), newCallToolRequest(repoArgs(map[string]any{"run_id": float64(99)})))
	if err == nil && (res == nil || !res.IsError) {
		t.Fatalf("unknown run must be an error, got %+v", res)
	}
}
