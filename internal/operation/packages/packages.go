// SPDX-License-Identifier: GPL-3.0-or-later

package packages

import (
	"context"
	"errors"

	apppkg "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/packages"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/packagesadapter"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/log"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/decode"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcpresult"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/params"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/to"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	ListPackagesToolName     = "list_packages"
	GetPackageToolName       = "get_package"
	DeletePackageToolName    = "delete_package"
	ListPackageFilesToolName = "list_package_files"

	defaultPackagesLimit = 30
	maxPackagesLimit     = 50
)

var (
	GetPackageTool = mcp.NewTool(
		GetPackageToolName,
		mcp.WithDescription("Get one package version (id, type, name, version, html_url, created_at, repository full_name). Does not embed owner or creator users."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("type", mcp.Required(), mcp.Description(params.PackageType)),
		mcp.WithString("name", mcp.Required(), mcp.Description(params.PackageName)),
		mcp.WithString("version", mcp.Required(), mcp.Description(params.PackageVersion)),
	)

	DeletePackageTool = mcp.NewTool(
		DeletePackageToolName,
		mcp.WithDescription("Delete one package version. Removes that version only, not every version of the name. No preflight GET. HTTP 204 returns {owner, type, name, version, status: \"deleted\"}; 4xx/5xx stay errors."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("type", mcp.Required(), mcp.Description(params.PackageType)),
		mcp.WithString("name", mcp.Required(), mcp.Description(params.PackageName)),
		mcp.WithString("version", mcp.Required(), mcp.Description(params.PackageVersion)),
	)
)

// RegisterTool registers the package tools. Each call takes its Forgejo port
// from f, so it runs under the credential of the request that made it.
func RegisterTool(s *server.MCPServer, f forge.Factory) {
	h := handlers{forge: f}
	s.AddTool(ListPackagesTool, h.ListPackagesFn)
	s.AddTool(GetPackageTool, h.GetPackageFn)
	s.AddTool(DeletePackageTool, h.DeletePackageFn)
	s.AddTool(ListPackageFilesTool, h.ListPackageFilesFn)
}

// handlers serve the package tools. They hold the factory rather than a port
// so each request gets its own credential.
type handlers struct{ forge forge.Factory }

// Argument structs, one per tool. `json` names the MCP argument and
// `required:"true"` marks the required ones; mcptest.ArgsMatchSchema keeps
// them in agreement with the tool definitions. A page or limit is a decode.Arg,
// because a wrongly typed one is an error rather than the default.

type packageArgs struct {
	Owner   string `json:"owner" required:"true"`
	Type    string `json:"type" required:"true"`
	Name    string `json:"name" required:"true"`
	Version string `json:"version" required:"true"`
}

type listPackagesArgs struct {
	Owner string     `json:"owner" required:"true"`
	Type  string     `json:"type"`
	Q     string     `json:"q"`
	Page  decode.Arg `json:"page" schema:"number"`
	Limit decode.Arg `json:"limit" schema:"number"`
}

type listPackageFilesArgs struct {
	Owner   string     `json:"owner" required:"true"`
	Type    string     `json:"type" required:"true"`
	Name    string     `json:"name" required:"true"`
	Version string     `json:"version" required:"true"`
	Page    decode.Arg `json:"page" schema:"number"`
	Limit   decode.Arg `json:"limit" schema:"number"`
}

// coords checks the coordinates in the order the tools always have: owner,
// type, name, version. An empty value is a missing one.
func (a packageArgs) coords() (apppkg.Coords, error) {
	if a.Owner == "" {
		return apppkg.Coords{}, errors.New("owner is required")
	}
	if a.Type == "" {
		return apppkg.Coords{}, errors.New("type is required")
	}
	if a.Name == "" {
		return apppkg.Coords{}, errors.New("name is required")
	}
	if a.Version == "" {
		return apppkg.Coords{}, errors.New("version is required")
	}
	return apppkg.Coords{Owner: a.Owner, Type: a.Type, Name: a.Name, Version: a.Version}, nil
}

// GetPackageFn validates the coordinates, then runs the get use case.
func (h handlers) GetPackageFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetPackageFn")
	var a packageArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	c, err := a.coords()
	if err != nil {
		return to.ErrorResult(err)
	}

	port, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	return mcpresult.Render(apppkg.New(packagesadapter.New(port)).Get(ctx, c))
}

// DeletePackageFn validates the coordinates, then runs the delete use case.
func (h handlers) DeletePackageFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeletePackageFn")
	var a packageArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	c, err := a.coords()
	if err != nil {
		return to.ErrorResult(err)
	}

	port, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	return mcpresult.Render(apppkg.New(packagesadapter.New(port)).Delete(ctx, c))
}
