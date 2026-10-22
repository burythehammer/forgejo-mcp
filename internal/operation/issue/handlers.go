// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"

	apppissue "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/issue"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/issueadapter"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/log"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/decode"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcpresult"

	"github.com/mark3labs/mcp-go/mcp"
)

// handlers serve the issue, label and dependency tools. They hold the factory
// rather than a port so each call is bound to the credential of its own
// request. Each handler is the MCP adapter of one use case in internal/app/issue:
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
	if v, ok := any(&args).(validator); ok {
		if err := v.validate(); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
	}
	port, err := h.forge.For(ctx)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return do(ctx, apppissue.New(issueadapter.New(port)), args)
}

// validator is implemented by an argument struct whose numbers are checked by
// the numeric-argument module before its handler runs.
type validator interface{ validate() error }
