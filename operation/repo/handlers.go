// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"

	apprepo "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/repo"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/decode"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/mcpresult"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/repoadminadapter"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"

	"github.com/mark3labs/mcp-go/mcp"
)

// handlers serve the repo tools; they hold the factory so each call uses its own request's credential.
type handlers struct{ forge forge.Factory }

// runAdmin is the shared shape of a repository-administration tool call:
// decode the arguments into A, take the use cases bound to this request's
// credential, and hand both to do, which runs one use case and renders it.
func runAdmin[A any](ctx context.Context, h handlers, req mcp.CallToolRequest, name string,
	do func(context.Context, *apprepo.AdminService, A) (*mcp.CallToolResult, error),
) (*mcp.CallToolResult, error) {
	log.Debugf("Called %s", name)
	var args A
	if err := decode.Lax(req, &args); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	port, err := h.forge.For(ctx)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return do(ctx, apprepo.NewAdmin(repoadminadapter.New(port)), args)
}

// pageOr is a page or limit argument as the tools have always read it: an
// absent or zero number takes the default. The test is made before the number
// is truncated, so 0.5 is not defaulted; it truncates to 0 and is passed on.
func pageOr(v float64, def int) int {
	if v == 0 {
		return def
	}
	return int(v)
}
