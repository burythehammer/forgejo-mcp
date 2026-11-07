// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"

	apppissue "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/issue"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/issueadapter"
)

// handlers serve the issue, label and dependency tools. They hold the factory
// rather than a port so each call is bound to the credential of its own
// request. Each handler is the MCP adapter of one use case in internal/app/issue:
// decode the arguments, run the use case, render the result.
type handlers struct{ forge forge.Factory }

func newHandlers(f forge.Factory) handlers { return handlers{forge: f} }

// service is the Binder for the issue, label, milestone and dependency tools:
// it looks up the request's credential and binds the issue use case to it. The
// pipeline calls it only after the request is valid.
func (h handlers) service(ctx context.Context) (*apppissue.Service, error) {
	port, err := h.forge.For(ctx)
	if err != nil {
		return nil, err
	}
	return apppissue.New(issueadapter.New(port)), nil
}
