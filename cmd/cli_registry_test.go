// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/resource"

	"github.com/mark3labs/mcp-go/server"
)

// The CLI registers tools through its own domain loop, not operation.RegisterTool.
// It must still record the registry, or a truncation note built by a tool
// loses its pointer (resource.ToolRef returns "" for every name).
func TestRegisterToolsWithDomains_RecordsRegistry(t *testing.T) {
	t.Cleanup(func() { resource.SetRegisteredTools(nil) })
	resource.SetRegisteredTools(nil)

	s := server.NewMCPServer("test", "0")
	registerToolsWithDomains(s, "test", fake.Factory{Forgejo: fake.New()})

	if got := resource.ToolRef("get_wiki_page"); got != "get_wiki_page" {
		t.Errorf("ToolRef(registered tool) = %q, want the name unchanged", got)
	}
	if got := resource.ToolRef("tool_that_was_never_registered"); got != "" {
		t.Errorf("ToolRef(unregistered tool) = %q, want empty", got)
	}
}
