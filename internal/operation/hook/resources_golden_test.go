// SPDX-License-Identifier: GPL-3.0-or-later

package hook

import (
	"context"
	"testing"
	"time"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcptest"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/resource"

	"github.com/mark3labs/mcp-go/mcp"
)

// TestHooksResourceGolden_CapAndSentinel pins what the hooks collection resource
// does on purpose: it stops at 30 hooks, says so with the shared sentinel, and
// names list_repo_hooks. The tool returns the whole list (see golden_test.go).
func TestHooksResourceGolden_CapAndSentinel(t *testing.T) {
	resource.SetRegisteredTools([]string{ListRepoHooksToolName})
	t.Cleanup(func() { resource.SetRegisteredTools(nil) })

	h, f := newFixture(t)
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	f.Now = func() time.Time { return at }
	for i := 0; i < resource.EmbeddedListCap+1; i++ {
		seedHook(t, f, "https://example.test/h")
	}
	contents, err := h.hooksResourceHandler(context.Background(),
		mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "forgejo://repo/goern/forgejo-mcp/hooks"}})
	if err != nil {
		t.Fatal(err)
	}
	mcptest.Golden(t, "resource_hooks_cap_sentinel", []byte(contents[0].(mcp.TextResourceContents).Text))
}
