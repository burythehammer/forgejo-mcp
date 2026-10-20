// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge/forgetest"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge/sdk"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// TestRefusedToolsReportForgejoMessage runs the file and branch deletes through
// the real SDK adapter against the HTTP stub, with Forgejo refusing the call. The
// tool must report the status and Forgejo's message, not a bare status.
func TestRefusedToolsReportForgejoMessage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		tool    string
		args    map[string]any
		op      string // the port method Forgejo refuses
		status  int
		message string
	}{
		{"delete_file", DeleteFileToolName,
			map[string]any{"owner": "octo", "repo": "hello", "filePath": "docs/a.md", "message": "rm",
				"branch_name": "main", "sha": "0123456789abcdef0123456789abcdef01234567"},
			"DeleteFile", 422, "sha does not match the file"},
		{"delete_branch", DeleteBranchToolName,
			map[string]any{"owner": "octo", "repo": "hello", "branch": "feat"},
			"DeleteBranch", 404, "branch does not exist"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			stub := forgetest.NewStub(t)
			stub.Fake.AddRepo("octo", "hello")
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
				got = fmt.Sprint(res.Content)
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
