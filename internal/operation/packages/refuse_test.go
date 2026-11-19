// SPDX-License-Identifier: GPL-3.0-or-later

package packages

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge/fake"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// A tool whose credential the factory refuses reports that refusal, once its
// arguments are valid, and never reaches Forgejo.
func TestToolsRefuseWhenTheFactoryRefuses(t *testing.T) {
	t.Parallel()
	refuse := fmt.Errorf("no credential")
	s := server.NewMCPServer("test", "0")
	RegisterTool(s, fake.Factory{Err: refuse})
	req := mcp.CallToolRequest{Params: mcp.CallToolParams{Name: ListPackagesToolName, Arguments: map[string]any{"owner": "octo"}}}
	_, err := s.GetTool(ListPackagesToolName).Handler(context.Background(), req)
	if err == nil || !strings.Contains(err.Error(), "no credential") {
		t.Fatalf("got %v, want the factory's error", err)
	}
}

// A bad request is refused as a tool error naming the missing argument, before
// the credential is looked up, so a refused credential never shows up for it.
func TestBadRequestIsRefusedBeforeTheCredential(t *testing.T) {
	t.Parallel()
	refuse := fmt.Errorf("no credential")
	s := server.NewMCPServer("test", "0")
	RegisterTool(s, fake.Factory{Err: refuse})
	req := mcp.CallToolRequest{Params: mcp.CallToolParams{Name: GetPackageToolName, Arguments: map[string]any{"owner": "octo"}}}
	res, err := s.GetTool(GetPackageToolName).Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("got Go error %v, want a tool error", err)
	}
	if !res.IsError || !strings.Contains(res.Content[0].(mcp.TextContent).Text, "type is required") {
		t.Fatalf("got %#v, want a tool error naming type", res)
	}
}
