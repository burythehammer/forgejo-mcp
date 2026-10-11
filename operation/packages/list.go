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

var ListPackagesTool = mcp.NewTool(
	ListPackagesToolName,
	mcp.WithDescription("List package versions for a user or org (Forgejo SearchVersions: one row per version, not one per name). Optional type and q filter; page (default 1) and limit (default 30, maximum 50) are sent as query parameters. Returns {packages, page, limit, count, has_next} and total_count when Forgejo sets X-Total-Count. has_next is Link rel=next. A missing owner is an error, not an empty list."),
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
	args := req.GetArguments()

	owner, err := requiredOwner(args)
	if err != nil {
		return to.ErrorResult(err)
	}
	page, err := to.BoundedInt(args, "page", 1, 1, math.MaxInt32)
	if err != nil {
		return to.ErrorResult(err)
	}
	limit, err := to.BoundedInt(args, "limit", defaultPackagesLimit, 1, maxPackagesLimit)
	if err != nil {
		return to.ErrorResult(err)
	}
	packageType, _ := args["type"].(string)
	q, _ := args["q"].(string)

	port, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	return render(apppkg.New(packagesadapter.New(port)).List(ctx, apppkg.ListRequest{
		Owner: owner, Type: packageType, Query: q, Page: page, Limit: limit,
	}))
}
