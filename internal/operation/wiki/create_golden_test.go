// SPDX-License-Identifier: GPL-3.0-or-later

package wiki

import (
	"context"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcptest"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// The create golden runs over the in-memory Forgejo, not the stub: whether a
// create is refused depends on the pages that already exist, which the stub
// does not model. It pins the create tool's text, its refusal and that a
// refused create leaves the page as it was.

// runToolText runs a registered tool and returns what the client would see: the
// text, and whether the result is an error.
func runToolText(t *testing.T, s *server.MCPServer, tool string, args map[string]any) (string, bool) {
	t.Helper()
	st := s.GetTool(tool)
	if st == nil {
		t.Fatalf("tool %s not registered", tool)
	}
	base := map[string]any{"owner": testOwner, "repo": testRepo}
	for k, v := range args {
		base[k] = v
	}
	res, err := st.Handler(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Name: tool, Arguments: base}})
	if err != nil {
		return err.Error(), true
	}
	return res.Content[0].(mcp.TextContent).Text, res.IsError
}

func TestCreateOverFakeForgejo(t *testing.T) {
	t.Parallel()
	f := fake.New()
	f.AddRepo(testOwner, testRepo)
	s := server.NewMCPServer("test", "0")
	RegisterTool(s, fake.Factory{Forgejo: f})

	var out strings.Builder
	step := func(label, tool string, args map[string]any) (string, bool) {
		text, isErr := runToolText(t, s, tool, args)
		out.WriteString(label + ": ")
		if isErr {
			out.WriteString("ERROR ")
		}
		out.WriteString(text + "\n")
		return text, isErr
	}

	step("create new title", CreateWikiPageToolName, map[string]any{"title": "Home", "content": "one"})

	_, isErr := step("create existing title", CreateWikiPageToolName, map[string]any{"title": "Home", "content": "two"})
	if !isErr {
		t.Fatal("create of an existing title was not refused")
	}

	read, _ := step("read after refused create", GetWikiPageToolName, map[string]any{"page_name": "Home"})
	if !strings.Contains(read, "one") || strings.Contains(read, "two") {
		t.Fatalf("page changed by a refused create: %q", read)
	}

	step("create guides", CreateWikiPageToolName, map[string]any{"title": "Guides/Setup", "content": "a"})
	_, isErr = step("create percent-encoded variant", CreateWikiPageToolName, map[string]any{"title": "Guides%2FSetup", "content": "b"})
	if !isErr {
		t.Fatal("percent-encoded variant of an existing title was not refused")
	}

	mcptest.Golden(t, "create_over_fake", []byte(out.String()))
}
