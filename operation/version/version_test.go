// SPDX-License-Identifier: GPL-3.0-or-later

package version

import (
	"context"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func callVersion(t *testing.T, version string) string {
	t.Helper()
	s := server.NewMCPServer("test", "0")
	RegisterTool(s, version)
	tool := s.GetTool(GetForgejoMCPServerVersion)
	if tool == nil {
		t.Fatal("version tool not registered")
	}
	res, err := tool.Handler(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatal(err)
	}
	return res.Content[0].(mcp.TextContent).Text
}

func TestVersionToolReportsTheVersionItWasRegisteredWith(t *testing.T) {
	if got := callVersion(t, "1.2.3"); !strings.Contains(got, "1.2.3") {
		t.Fatalf("got %q, want it to name 1.2.3", got)
	}
}

func TestVersionToolFallsBackToDev(t *testing.T) {
	if got := callVersion(t, ""); !strings.Contains(got, "dev") {
		t.Fatalf("got %q, want dev", got)
	}
}
