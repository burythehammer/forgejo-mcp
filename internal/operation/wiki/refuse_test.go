// SPDX-License-Identifier: GPL-3.0-or-later

package wiki

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge/fake"

	"github.com/mark3labs/mcp-go/mcp"
)

// A tool whose credential the factory refuses reports that refusal, and never
// reaches Forgejo.
func TestToolsRefuseWhenTheFactoryRefuses(t *testing.T) {
	t.Parallel()
	refuse := fmt.Errorf("no credential")
	h := handlers{forge: fake.Factory{Err: refuse}}
	req := mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"owner": testOwner, "repo": testRepo}}}
	_, err := callTool(context.Background(), h, ListWikiPagesToolName, req)
	if err == nil || !strings.Contains(err.Error(), "no credential") {
		t.Fatalf("got %v, want the factory's error", err)
	}
}
