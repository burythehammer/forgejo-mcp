// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"

	apppissue "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/issue"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/decode"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/mcpresult"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/issueadapter"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"

	"github.com/mark3labs/mcp-go/mcp"
)

// handlers serve the issue, label and dependency tools. They hold the factory
// rather than a port so each call is bound to the credential of its own
// request. Each handler is the MCP adapter of one use case in app/issue:
// decode the arguments, run the use case, render the result.
type handlers struct{ forge forge.Factory }

func newHandlers(f forge.Factory) handlers { return handlers{forge: f} }

// run is the shared shape of a tool call: decode the arguments into A, take the
// use cases bound to this request's credential, and hand both to do, which
// runs one use case and renders its result.
func run[A any](ctx context.Context, h handlers, req mcp.CallToolRequest, name string,
	do func(context.Context, *apppissue.Service, A) (*mcp.CallToolResult, error),
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
	return do(ctx, apppissue.New(issueadapter.New(port)), args)
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

// intOr is pageOr for the dependency tools, whose defaults were always applied
// to the truncated number.
func intOr(v float64, def int) int {
	if n := int(v); n != 0 {
		return n
	}
	return def
}
