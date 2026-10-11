package version

import (
	"context"
	"fmt"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/to"

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
		v := version
		if v == "" {
			v = "dev"
		}
		return to.TextResult(fmt.Sprintf("Forgejo MCP Server version: %v", v))
	}
}
