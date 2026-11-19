// SPDX-License-Identifier: GPL-3.0-or-later

package version

import (
	"context"
	"errors"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/mcptest"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func TestMCPServerVersionOutputGolden(t *testing.T) {
	t.Parallel()
	mcptest.Golden(t, "version_1.2.3", []byte(callTool(t, GetForgejoMCPServerVersion, fake.Factory{}, "1.2.3")))
	mcptest.Golden(t, "version_dev", []byte(callTool(t, GetForgejoMCPServerVersion, fake.Factory{}, "")))
}

func TestForgejoVersionOutputGolden(t *testing.T) {
	t.Parallel()
	mcptest.Golden(t, "get_forgejo_version_16.0.0", []byte(callTool(t, GetForgejoVersionToolName, fake.Factory{}, "")))
	mcptest.Golden(t, "get_forgejo_version_17.0.0-rc1", []byte(callTool(t, GetForgejoVersionToolName, fake.Factory{Version: "17.0.0-rc1"}, "")))
}

func TestVersionSchemaSnapshot(t *testing.T) {
	t.Parallel()
	mcptest.SchemaSnapshot(t, GetForgejoMCPServerVersionTool)
}

func TestForgejoVersionSchemaSnapshot(t *testing.T) {
	t.Parallel()
	mcptest.SchemaSnapshot(t, GetForgejoVersionTool)
}

func TestVersionArgsMatchSchema(t *testing.T) {
	t.Parallel()
	mcptest.ArgsMatchSchema(t, GetForgejoMCPServerVersionTool, Args{})
}

func TestForgejoVersionArgsMatchSchema(t *testing.T) {
	t.Parallel()
	mcptest.ArgsMatchSchema(t, GetForgejoVersionTool, Args{})
}

// errProbe is what a failing Forgejo version probe returns.
var errProbe = errors.New("forgejo unreachable")

// failingProbe is a Forgejo factory whose version probe fails.
type failingProbe struct{ fake.Factory }

func (failingProbe) ProbeServerVersion(context.Context) (string, error) {
	return "", errProbe
}

func TestForgejoVersionProbeErrorIsAToolError(t *testing.T) {
	t.Parallel()
	s := server.NewMCPServer("test", "0")
	RegisterTool(s, "", failingProbe{})
	tool := s.GetTool(GetForgejoVersionToolName)
	if tool == nil {
		t.Fatal("get_forgejo_version not registered")
	}
	res, err := tool.Handler(context.Background(), mcp.CallToolRequest{})
	if !errors.Is(err, errProbe) {
		t.Fatalf("err = %v, want the probe error", err)
	}
	if res != nil {
		t.Fatalf("result = %+v, want none on a probe error", res)
	}
}

// callTool runs the named tool of a server registered with version and f, and
// returns its text.
func callTool(t *testing.T, name string, f forge.Factory, version string) string {
	t.Helper()
	s := server.NewMCPServer("test", "0")
	RegisterTool(s, version, f)
	tool := s.GetTool(name)
	if tool == nil {
		t.Fatalf("%s not registered", name)
	}
	res, err := tool.Handler(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatal(err)
	}
	return res.Content[0].(mcp.TextContent).Text
}
