// SPDX-License-Identifier: GPL-3.0-or-later

package packages

import (
	"context"
	"errors"

	apppkg "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/packages"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/packagesadapter"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/annotate"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/decode"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcpresult"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/params"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/pipeline"

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
		annotate.Read(),
		mcp.WithDescription("Get one package version (id, type, name, version, html_url, created_at, repository full_name). Does not embed owner or creator users."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("type", mcp.Required(), mcp.Description(params.PackageType)),
		mcp.WithString("name", mcp.Required(), mcp.Description(params.PackageName)),
		mcp.WithString("version", mcp.Required(), mcp.Description(params.PackageVersion)),
	)

	DeletePackageTool = mcp.NewTool(
		DeletePackageToolName,
		annotate.Destructive(),
		mcp.WithDescription("Delete one package version. Removes that version only, not every version of the name. No preflight GET. HTTP 204 returns {owner, type, name, version, status: \"deleted\"}; 4xx/5xx stay errors."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("type", mcp.Required(), mcp.Description(params.PackageType)),
		mcp.WithString("name", mcp.Required(), mcp.Description(params.PackageName)),
		mcp.WithString("version", mcp.Required(), mcp.Description(params.PackageVersion)),
	)
)

// RegisterTool registers the package tools. Each request takes its Forgejo
// port from f through the request pipeline's Binder, so it runs under the
// credential of the request that made it, and only after the request is valid.
func RegisterTool(s *server.MCPServer, f forge.Factory) {
	h := handlers{forge: f}
	s.AddTool(ListPackagesTool, pipeline.Handle(h.service, listPackages))
	s.AddTool(GetPackageTool, pipeline.Handle(h.service, getPackage))
	s.AddTool(DeletePackageTool, pipeline.Handle(h.service, deletePackage))
	s.AddTool(ListPackageFilesTool, pipeline.Handle(h.service, listPackageFiles))
}

// handlers holds the factory rather than a port so each request gets its own
// credential.
type handlers struct{ forge forge.Factory }

// service is the Binder for every package tool: it looks up the request's
// credential and binds the package use case to it.
func (h handlers) service(ctx context.Context) (*apppkg.Service, error) {
	port, err := h.forge.For(ctx)
	if err != nil {
		return nil, err
	}
	return apppkg.New(packagesadapter.New(port)), nil
}

// Argument structs, one per tool. `json` names the MCP argument and
// `required:"true"` marks the required ones; mcptest.ArgsMatchSchema keeps
// them in agreement with the tool definitions. A page or limit is a decode.Arg,
// because a wrongly typed one is an error rather than the default. Each struct
// with a Validate method is checked by the pipeline before the credential
// lookup.

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

// Validate checks the coordinates in the order the tools always have: owner,
// type, name, version. An empty value is a missing one.
func (a packageArgs) Validate() error {
	if a.Owner == "" {
		return errors.New("owner is required")
	}
	if a.Type == "" {
		return errors.New("type is required")
	}
	if a.Name == "" {
		return errors.New("name is required")
	}
	if a.Version == "" {
		return errors.New("version is required")
	}
	return nil
}

func (a packageArgs) coords() apppkg.Coords {
	return apppkg.Coords{Owner: a.Owner, Type: a.Type, Name: a.Name, Version: a.Version}
}

func getPackage(ctx context.Context, svc *apppkg.Service, a packageArgs) (*mcp.CallToolResult, error) {
	return mcpresult.Render(svc.Get(ctx, a.coords()))
}

func deletePackage(ctx context.Context, svc *apppkg.Service, a packageArgs) (*mcp.CallToolResult, error) {
	return mcpresult.Render(svc.Delete(ctx, a.coords()))
}
