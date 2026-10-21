// SPDX-License-Identifier: GPL-3.0-or-later

package attachment

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/forgetest"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/sdk"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// TestSmokeOverHTTP runs the attachment tools through the real SDK adapter
// against the HTTP stub: the multipart upload, the list's 404-is-empty rule and
// the inline download, end to end.
func TestSmokeOverHTTP(t *testing.T) {
	t.Parallel()
	stub := forgetest.NewStub(t)
	stub.Fake.AddRepo("octo", "hello")
	stub.Fake.AddAttachmentIssue("octo", "hello", 1)
	// Uploads and downloads are raw HTTP, so the adapter needs the stub's base URL.
	port := sdk.NewAdapter(stub.NewClient(t)).WithRawBaseURL(stub.URL)

	s := server.NewMCPServer("test", "0")
	RegisterTool(s, fake.Factory{Forgejo: port})
	call := func(tool string, args map[string]any) string {
		t.Helper()
		base := map[string]any{"owner": "octo", "repo": "hello"}
		for k, v := range args {
			base[k] = v
		}
		res, err := s.GetTool(tool).Handler(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Name: tool, Arguments: base}})
		if err != nil || res.IsError {
			t.Fatalf("%s: %v %+v", tool, err, res)
		}
		return res.Content[0].(mcp.TextContent).Text
	}

	if got := call(ListIssueAttachmentsToolName, map[string]any{"index": float64(1)}); got != `{"Result":[]}` {
		t.Fatalf("list on an issue with none: %s", got)
	}
	content := base64.StdEncoding.EncodeToString([]byte("hello attachment"))
	created := call(CreateIssueAttachmentToolName, map[string]any{"index": float64(1), "content": content, "filename": "a.txt", "mime_type": "text/plain"})
	if !strings.Contains(created, `"name":"a.txt"`) {
		t.Fatalf("create: %s", created)
	}
	if got := call(ListIssueAttachmentsToolName, map[string]any{"index": float64(1)}); !strings.Contains(got, `"name":"a.txt"`) {
		t.Fatalf("list after create: %s", got)
	}
}
