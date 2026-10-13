// SPDX-License-Identifier: GPL-3.0-or-later

package release

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

// TestSmokeOverHTTP runs the release tools through the real SDK adapter against
// the HTTP stub: the request paths, decoding and error mapping end to end.
func TestSmokeOverHTTP(t *testing.T) {
	t.Parallel()
	stub := forgetest.NewStub(t)
	stub.Fake.AddRepo("octo", "hello")
	port := sdk.NewAdapter(stub.NewClient(t))

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

	if got := call(ListReleasesToolName, nil); got != `{"Result":{"releases":[],"page":1,"limit":20,"count":0,"has_next":false}}` {
		t.Fatalf("list on an empty repo: %s", got)
	}
	if got := call(CreateReleaseToolName, map[string]any{"tag_name": "v1.0.0"}); !strings.Contains(got, `"tag_name":"v1.0.0"`) {
		t.Fatalf("create: %s", got)
	}
	if got := call(GetReleaseByTagToolName, map[string]any{"tag": "v1.0.0"}); !strings.Contains(got, `"name":"v1.0.0"`) {
		t.Fatalf("get by tag: %s", got)
	}
	if got := call(ListReleaseAttachmentsToolName, map[string]any{"release_id": float64(1)}); got == "" {
		t.Fatalf("list attachments returned nothing")
	}
}
