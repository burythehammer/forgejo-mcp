// SPDX-License-Identifier: GPL-3.0-or-later

package version

import (
	"context"
	"fmt"

	appversion "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/version"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/operation/decode"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/log"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/to"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	GetForgejoMCPServerVersion = "get_forgejo_mcp_server_version"
)

var GetForgejoMCPServerVersionTool = mcp.NewTool(
	GetForgejoMCPServerVersion,
	mcp.WithDescription("Get MCP server version"),
)

// RegisterTool registers the version tool, which reports version.
func RegisterTool(s *server.MCPServer, version string) {
	s.AddTool(GetForgejoMCPServerVersionTool, GetForgejoMCPServerVersionFn(version))
}

func GetForgejoMCPServerVersionFn(version string) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		log.Debugf("Called GetForgejoMCPServerVersionFn")
		var args Args
		if err := decode.Args(req, &args); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		res := appversion.Run(version)
		return to.TextResult(fmt.Sprintf("Forgejo MCP Server version: %v", res.Version))
	}
}

// Args is the version tool's argument struct. It takes none.
type Args struct{}
