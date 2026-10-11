// SPDX-License-Identifier: GPL-3.0-or-later

package version

import (
	"context"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/mcptest"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func TestVersionOutputGolden(t *testing.T) {
	mcptest.Golden(t, "version_1.2.3", []byte(callVersion(t, "1.2.3")))
	mcptest.Golden(t, "version_dev", []byte(callVersion(t, "")))
}

func TestVersionSchemaSnapshot(t *testing.T) {
	mcptest.SchemaSnapshot(t, GetForgejoMCPServerVersionTool)
}

func TestVersionArgsMatchSchema(t *testing.T) {
	mcptest.ArgsMatchSchema(t, GetForgejoMCPServerVersionTool, Args{})
}

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
