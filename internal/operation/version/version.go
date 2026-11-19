// SPDX-License-Identifier: GPL-3.0-or-later

package version

import (
	"context"
	"fmt"

	appversion "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/version"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/annotate"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/pipeline"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/to"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	GetForgejoMCPServerVersion = "get_forgejo_mcp_server_version"
)

var GetForgejoMCPServerVersionTool = mcp.NewTool(
	GetForgejoMCPServerVersion,
	annotate.Read(),
	mcp.WithDescription("Get MCP server version"),
)

// RegisterTool registers the version tool, which reports version.
func RegisterTool(s *server.MCPServer, version string) {
	s.AddTool(GetForgejoMCPServerVersionTool, GetForgejoMCPServerVersionFn(version))
}

// GetForgejoMCPServerVersionFn is the version tool's handler. It needs no
// credential, so its Binder yields nothing, and it decodes strictly, as the
// hand-written handler did.
func GetForgejoMCPServerVersionFn(version string) server.ToolHandlerFunc {
	return pipeline.Handle(noCredential, func(_ context.Context, _ struct{}, _ Args) (*mcp.CallToolResult, error) {
		res := appversion.Run(version)
		return to.TextResult(fmt.Sprintf("Forgejo MCP Server version: %v", res.Version))
	}, pipeline.Strict())
}

func noCredential(context.Context) (struct{}, error) { return struct{}{}, nil }

// Args is the version tool's argument struct. It takes none.
type Args struct{}
