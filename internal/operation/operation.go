// SPDX-License-Identifier: GPL-3.0-or-later

package operation

import (
	"context"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/config"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/sdk"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/log"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/actions"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/attachment"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/branchprotection"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/hook"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/issue"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/org"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/packages"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/pull"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/release"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/repo"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/resource"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/search"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/tracking"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/user"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/version"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/wiki"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/toolset"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/transport"

	"github.com/mark3labs/mcp-go/server"
)

// RegisterTool registers every tool; version is what the version tool reports.
// The Forgejo factory is built once by the composition root; a domain takes it
// when it migrates to the Forgejo port.
func RegisterTool(s *server.MCPServer, version string, f forge.Factory) {
	RegisterSelectedTool(s, version, f, toolset.Everything())
}

// RegisterSelectedTool registers every tool, then removes the ones sel does not
// hold, so tools/list carries only the operator's selection. Each domain's
// register function adds its tools to the server directly and gives no
// per-tool hook, so the removal happens here, before the server serves
// anything. The result is the same as never registering them.
//
// To put a tool in a toolset, add its name to internal/toolset; the registry
// test fails for a tool in no toolset.
func RegisterSelectedTool(s *server.MCPServer, version string, f forge.Factory, sel toolset.Selection) {
	log.Info("Registering MCP tools")

	RegisterUserTool(s, f)
	RegisterRepoTool(s, f)
	RegisterIssueTool(s, f)
	RegisterPullTool(s, f)
	RegisterPullReviewTool(s, f)
	RegisterSearchTool(s, f)
	RegisterVersionTool(s, version)
	RegisterActionsTool(s, f)
	RegisterOrgTool(s, f)
	RegisterPackagesTool(s, f)
	RegisterTrackingTool(s, f)
	RegisterAttachmentTool(s, f)
	RegisterReleaseTool(s, f)
	RegisterBranchProtectionTool(s, f)
	RegisterHookTool(s, f)
	RegisterWikiTool(s, f)

	if !sel.Full() {
		var drop []string
		for name := range s.ListTools() {
			if !sel.HasTool(name) {
				drop = append(drop, name)
			}
		}
		if len(drop) > 0 {
			s.DeleteTools(drop...)
		}
	}

	// Truncation notes may name only tools in this registry (resource.ToolRef).
	tools := s.ListTools()
	names := make([]string, 0, len(tools))
	for name := range tools {
		names = append(names, name)
	}
	resource.SetRegisteredTools(names)

	log.Info("All MCP tools registered successfully")
}

// Per-domain registration functions exposed for CLI domain grouping.

func RegisterUserTool(s *server.MCPServer, f forge.Factory) {
	user.RegisterTool(s, f)
	log.Debug("Registered user tools")
}

func RegisterRepoTool(s *server.MCPServer, f forge.Factory) {
	repo.RegisterTool(s, f)
	log.Debug("Registered repository tools")
}

func RegisterIssueTool(s *server.MCPServer, f forge.Factory) {
	issue.RegisterTool(s, f)
	log.Debug("Registered issue tools")
}

func RegisterPullTool(s *server.MCPServer, f forge.Factory) {
	pull.RegisterTool(s, f)
	log.Debug("Registered pull request tools")
}

func RegisterPullReviewTool(s *server.MCPServer, f forge.Factory) {
	pull.RegisterReviewTools(s, f)
	log.Debug("Registered pull review write tools")
}

func RegisterSearchTool(s *server.MCPServer, f forge.Factory) {
	search.RegisterTool(s, f)
	log.Debug("Registered search tools")
}

func RegisterVersionTool(s *server.MCPServer, ver string) {
	version.RegisterTool(s, ver)
	log.Debug("Registered version tools")
}

func RegisterActionsTool(s *server.MCPServer, f forge.Factory) {
	actions.RegisterTool(s, f)
	log.Debug("Registered actions tools")
}

func RegisterOrgTool(s *server.MCPServer, f forge.Factory) {
	org.RegisterTool(s, f)
	log.Debug("Registered org tools")
}

func RegisterPackagesTool(s *server.MCPServer, f forge.Factory) {
	packages.RegisterTool(s, f)
	log.Debug("Registered packages tools")
}

func RegisterTrackingTool(s *server.MCPServer, f forge.Factory) {
	tracking.RegisterTool(s, f)
	log.Debug("Registered time tracking tools")
}

func RegisterAttachmentTool(s *server.MCPServer, f forge.Factory) {
	attachment.RegisterTool(s, f)
	log.Debug("Registered attachment tools")
}

func RegisterReleaseTool(s *server.MCPServer, f forge.Factory) {
	release.RegisterTool(s, f)
	log.Debug("Registered release tools")
}

func RegisterBranchProtectionTool(s *server.MCPServer, f forge.Factory) {
	branchprotection.RegisterTool(s, f)
	log.Debug("Registered branch protection tools")
}

func RegisterHookTool(s *server.MCPServer, f forge.Factory) {
	hook.RegisterTools(s, f)
	log.Debug("Registered webhook tools")
}

func RegisterWikiTool(s *server.MCPServer, f forge.Factory) {
	wiki.RegisterTool(s, f)
	log.Debug("Registered wiki tools")
}

// Run builds the MCP server with every tool and resource registered, then hands
// it to the transport package to serve on cfg.Transport. version is the build
// version, which is build information and so not part of cfg.
func Run(ctx context.Context, cfg config.Config, version string) error {
	sel, err := toolset.Parse(cfg.Toolsets, cfg.Tools)
	if err != nil {
		return err
	}
	mcpServer := newMCPServer(version)
	factory := newFactory(cfg, version)
	RegisterSelectedTool(mcpServer, version, factory, sel)
	RegisterSelectedResources(mcpServer, factory, sel)
	return transport.Serve(ctx, cfg, mcpServer, factory)
}

// newFactory builds the Forgejo factory from cfg. The factory holds no credential
// policy. The transport's authentication port decides whether a request may be
// served with the operator's credential and marks the request's context to say
// so. Stdio requests are always marked. A network request is marked only when
// the operator has opted into the fallback and the request carries no token of
// its own. The factory serves a marked request with its operator client and
// refuses any other request that carries no token.
func newFactory(cfg config.Config, version string) *sdk.Factory {
	return sdk.NewFactory(cfg.Forgejo, version)
}

func RegisterCoreResources(s *server.MCPServer, f forge.Factory) {
	RegisterSelectedResources(s, f, toolset.Everything())
}

// RegisterSelectedResources registers the resource templates whose domain
// toolset sel holds. A template follows its domain's toolset: the repository,
// commit and status templates follow repos, the owner template follows user.
func RegisterSelectedResources(s *server.MCPServer, f forge.Factory, sel toolset.Selection) {
	if sel.HasToolset("repos") {
		RegisterCommitResource(s, f)
		RegisterRepoResource(s, f)
		RegisterStatusResource(s, f)
	}
	if sel.HasToolset("user") {
		RegisterOwnerResource(s, f)
	}
	if sel.HasToolset("issues") {
		RegisterIssueResources(s, f)
	}
	if sel.HasToolset("labels") {
		RegisterLabelResources(s, f)
	}
	if sel.HasToolset("pulls") {
		RegisterPullResources(s, f)
	}
	if sel.HasToolset("branch-protection") {
		RegisterBranchProtectionResources(s, f)
	}
	if sel.HasToolset("webhooks") {
		RegisterHookResources(s, f)
	}
	if sel.HasToolset("wiki") {
		RegisterWikiResource(s, f)
	}
	log.Debug("Registered core resource templates")
}

func RegisterWikiResource(s *server.MCPServer, f forge.Factory) {
	wiki.RegisterWikiResource(s, f)
	log.Debug("Registered wiki resource template")
}

func RegisterLabelResources(s *server.MCPServer, f forge.Factory) {
	issue.RegisterLabelResources(s, f)
	log.Debug("Registered label resource templates")
}

func RegisterBranchProtectionResources(s *server.MCPServer, f forge.Factory) {
	branchprotection.RegisterResource(s, f)
	log.Debug("Registered branch protection resource templates")
}

func RegisterHookResources(s *server.MCPServer, f forge.Factory) {
	hook.RegisterResources(s, f)
	log.Debug("Registered webhook resource templates")
}

func RegisterPullResources(s *server.MCPServer, f forge.Factory) {
	pull.RegisterPullResources(s, f)
	log.Debug("Registered pull request resource template")
}

func RegisterIssueResources(s *server.MCPServer, f forge.Factory) {
	issue.RegisterIssueResources(s, f)
	log.Debug("Registered issue resource templates")
}

func RegisterCommitResource(s *server.MCPServer, f forge.Factory) {
	repo.RegisterCommitResource(s, f)
	log.Debug("Registered commit resource template")
}

func RegisterOwnerResource(s *server.MCPServer, f forge.Factory) {
	user.RegisterOwnerResource(s, f)
	log.Debug("Registered owner resource template")
}

func RegisterRepoResource(s *server.MCPServer, f forge.Factory) {
	repo.RegisterRepoResource(s, f)
	log.Debug("Registered repo resource template")
}

func RegisterStatusResource(s *server.MCPServer, f forge.Factory) {
	repo.RegisterStatusResource(s, f)
	log.Debug("Registered commit status resource template")
}

func newMCPServer(version string) *server.MCPServer {
	return server.NewMCPServer(
		"Forgejo MCP Server",
		version,
		server.WithLogging(),
		server.WithResourceCapabilities(false, false),
	)
}
