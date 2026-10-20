// SPDX-License-Identifier: GPL-3.0-or-later

package packages

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge/fake"

	"github.com/mark3labs/mcp-go/mcp"
)

// A tool whose credential the factory refuses reports that refusal, once its
// arguments are valid, and never reaches Forgejo.
func TestToolsRefuseWhenTheFactoryRefuses(t *testing.T) {
	t.Parallel()
	refuse := fmt.Errorf("no credential")
	h := handlers{forge: fake.Factory{Err: refuse}}
	req := mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{"owner": "octo"}}}
	_, err := h.ListPackagesFn(context.Background(), req)
	if err == nil || !strings.Contains(err.Error(), "no credential") {
		t.Fatalf("got %v, want the factory's error", err)
	}
}
