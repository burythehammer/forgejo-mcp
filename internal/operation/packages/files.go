// SPDX-License-Identifier: GPL-3.0-or-later

package packages

import (
	"context"

	apppkg "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/packages"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/packagesadapter"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/log"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/decode"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/params"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/to"

	"github.com/mark3labs/mcp-go/mcp"
)

var ListPackageFilesTool = mcp.NewTool(
	ListPackageFilesToolName,
	mcp.WithDescription("List files of one package version. Forgejo returns the full file list with no paging; page (default 1) and limit (default 30, maximum 50) slice it client-side. Returns {files, page, limit, count, has_next, total_count}. has_next true means a further page may exist; re-issue the call with page incremented to fetch it. total_count is the grand total: the number of files in the version (the files endpoint has no X-Total-Count, and its listing is read whole). Each file is id, name, size, sha256."),
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
	var a listPackageFilesArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	c, err := packageArgs{Owner: a.Owner, Type: a.Type, Name: a.Name, Version: a.Version}.coords()
	if err != nil {
		return to.ErrorResult(err)
	}
	page, limit, err := pageAndLimit(a.Page, a.Limit)
	if err != nil {
		return to.ErrorResult(err)
	}

	port, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	return listPackageFilesResult(apppkg.New(packagesadapter.New(port)).ListFiles(ctx, apppkg.ListFilesRequest{
		Coords: c,
		Page:   page,
		Limit:  limit,
	}))
}
