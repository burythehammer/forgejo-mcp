// SPDX-License-Identifier: GPL-3.0-or-later

package packages

import (
	"context"
	"errors"
	"math"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/list"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	apppkg "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/packages"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/packagesadapter"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/log"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/decode"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcpresult"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/params"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/to"

	"github.com/mark3labs/mcp-go/mcp"
)

var ListPackagesTool = mcp.NewTool(
	ListPackagesToolName,
	mcp.WithDescription("List package versions for a user or org (Forgejo SearchVersions: one row per version, not one per name). Optional type and q filter; page (default 1) and limit (default 30, maximum 50) are sent as query parameters. Returns {packages, page, limit, count, has_next} and total_count when Forgejo sets X-Total-Count. has_next true means a further page may exist: it is derived from total_count when Forgejo reports one, and otherwise from probing the next page. Re-issue the call with page incremented to fetch it. A missing owner is an error, not an empty list."),
	mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
	mcp.WithString("type", mcp.Description(params.PackageType)),
	mcp.WithString("q", mcp.Description(params.PackageQ)),
	mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
	mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(defaultPackagesLimit), mcp.Min(1), mcp.Max(maxPackagesLimit)),
)

// ListPackagesFn validates the arguments, then runs the list use case. The
// validation comes before the credential lookup, as it always has.
func (h handlers) ListPackagesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListPackagesFn")
	var a listPackagesArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	if a.Owner == "" {
		return to.ErrorResult(errors.New("owner is required"))
	}
	page, limit, err := pageAndLimit(a.Page, a.Limit)
	if err != nil {
		return to.ErrorResult(err)
	}

	port, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	return listPackagesResult(apppkg.New(packagesadapter.New(port)).List(ctx, apppkg.ListRequest{
		Owner: a.Owner, Type: a.Type, Query: a.Q, Page: page, Limit: limit,
	}))
}

// pageAndLimit reads the paging arguments. An absent limit is 0, which the use
// case replaces with the default lowered to the instance ceiling; page
// defaults to 1. A given value must be a whole number in range.
func pageAndLimit(page, limit decode.Arg) (int, int, error) {
	p, err := page.Int("page", 1, 1, math.MaxInt32)
	if err != nil {
		return 0, 0, err
	}
	l, err := limit.Int("limit", 0, 1, maxPackagesLimit)
	if err != nil {
		return 0, 0, err
	}
	return p, l, nil
}

// listPackagesResult renders one page of package versions in the list tool's wire form.
func listPackagesResult(b list.Bounded[model.PackageView], err error) (*mcp.CallToolResult, error) {
	if err != nil {
		return mcpresult.Fail(err)
	}
	return mcpresult.Render(model.ListPackagesResult{Packages: b.Items, PageMeta: model.PageMetaOf(b)}, nil)
}

// listPackageFilesResult renders one page of a version's files in the files tool's wire form.
func listPackageFilesResult(b list.Bounded[model.PackageFileView], err error) (*mcp.CallToolResult, error) {
	if err != nil {
		return mcpresult.Fail(err)
	}
	return mcpresult.Render(model.ListPackageFilesResult{Files: b.Items, PageMeta: model.PageMetaOf(b)}, nil)
}
