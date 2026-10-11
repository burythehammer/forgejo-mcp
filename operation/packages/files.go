// SPDX-License-Identifier: GPL-3.0-or-later

package packages

import (
	"context"
	"math"

	apppkg "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/packages"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/params"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/packagesadapter"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/to"

	"github.com/mark3labs/mcp-go/mcp"
)

var ListPackageFilesTool = mcp.NewTool(
	ListPackageFilesToolName,
	mcp.WithDescription("List files of one package version. Forgejo returns the full file list with no paging; page (default 1) and limit (default 30, maximum 50) slice it client-side. Returns {files, page, limit, count, has_next, total_count}. total_count is the fetched list length (the files endpoint has no X-Total-Count). Each file is id, name, size, sha256."),
	mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
	mcp.WithString("type", mcp.Required(), mcp.Description(params.PackageType)),
	mcp.WithString("name", mcp.Required(), mcp.Description(params.PackageName)),
	mcp.WithString("version", mcp.Required(), mcp.Description(params.PackageVersion)),
	mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
	mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(defaultPackagesLimit), mcp.Min(1), mcp.Max(maxPackagesLimit)),
)

// ListPackageFilesFn validates the arguments, then runs the files use case.
func (h handlers) ListPackageFilesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListPackageFilesFn")
	args := req.GetArguments()

	owner, packageType, name, version, err := requiredPackageCoords(args)
	if err != nil {
		return to.ErrorResult(err)
	}
	page, err := boundedIntegerArg(args, "page", 1, 1, math.MaxInt32)
	if err != nil {
		return to.ErrorResult(err)
	}
	limit, err := boundedIntegerArg(args, "limit", defaultPackagesLimit, 1, maxPackagesLimit)
	if err != nil {
		return to.ErrorResult(err)
	}

	port, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	return render(apppkg.New(packagesadapter.New(port)).ListFiles(ctx, apppkg.ListFilesRequest{
		Coords: apppkg.Coords{Owner: owner, Type: packageType, Name: name, Version: version},
		Page:   page,
		Limit:  limit,
	}))
}
