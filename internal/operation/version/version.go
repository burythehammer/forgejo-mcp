// SPDX-License-Identifier: GPL-3.0-or-later

package version

import (
	"context"
	"encoding/json"

	appversion "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/version"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/annotate"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/mcpresult"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/pipeline"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	GetForgejoMCPServerVersion = "get_forgejo_mcp_server_version"
	GetForgejoVersionToolName  = "get_forgejo_version"
)

var GetForgejoMCPServerVersionTool = mcp.NewTool(
	GetForgejoMCPServerVersion,
	annotate.Read(),
	mcp.WithDescription(`Get the MCP server version, as {"version": "..."}. This is the MCP server's own version, not the Forgejo version (see get_forgejo_version).`),
)

var GetForgejoVersionTool = mcp.NewTool(
	GetForgejoVersionToolName,
	annotate.Read(),
	mcp.WithDescription(`Get the Forgejo version of the configured instance, as {"version": "..."}, unparsed (for example 17.0.0-rc1). One request to the instance, which needs no credential.`),
)

// RegisterTool registers the version tools: version is the MCP server version
// that get_forgejo_mcp_server_version reports, and f is the Forgejo factory that
// get_forgejo_version probes.
func RegisterTool(s *server.MCPServer, version string, f forge.Factory) {
	s.AddTool(GetForgejoMCPServerVersionTool, GetForgejoMCPServerVersionFn(version))
	s.AddTool(GetForgejoVersionTool, GetForgejoVersionFn(f))
}

// GetForgejoMCPServerVersionFn is the MCP server version tool's handler. It needs
// no credential, so its Binder yields nothing, and it decodes strictly.
func GetForgejoMCPServerVersionFn(version string) server.ToolHandlerFunc {
	return pipeline.Handle(noCredential, func(_ context.Context, _ struct{}, _ Args) (*mcp.CallToolResult, error) {
		return versionResult(appversion.Run(version).Version)
	}, pipeline.Strict())
}

// GetForgejoVersionFn is the Forgejo version tool's handler. The probe sends no
// credential, so the Binder hands over the factory itself rather than
// f.For(ctx), which would refuse a request that carries no token.
func GetForgejoVersionFn(f forge.Factory) server.ToolHandlerFunc {
	bind := func(context.Context) (appversion.Prober, error) { return f, nil }
	return pipeline.Handle(bind, func(ctx context.Context, p appversion.Prober, _ Args) (*mcp.CallToolResult, error) {
		res, err := appversion.ProbeForgejo(ctx, p)
		if err != nil {
			return mcpresult.Fail(err)
		}
		return versionResult(res.Version)
	}, pipeline.Strict())
}

// versionBody is the JSON both version tools return.
type versionBody struct {
	Version string `json:"version"`
}

// versionResult sends {"version": v} as the tool's text. It is not wrapped in
// the {"Result": ...} envelope that to.TextResult adds.
func versionResult(v string) (*mcp.CallToolResult, error) {
	b, err := json.Marshal(versionBody{Version: v})
	if err != nil {
		return mcpresult.Fail(err)
	}
	return mcp.NewToolResultText(string(b)), nil
}

func noCredential(context.Context) (struct{}, error) { return struct{}{}, nil }

// Args is the argument struct of both version tools. It takes none.
type Args struct{}
