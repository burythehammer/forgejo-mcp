// SPDX-License-Identifier: GPL-3.0-or-later

// Package mcpresult turns a use case's outcome into the MCP tool result a handler
// returns. It is shared by every operation domain, so a use-case error reaches
// the client the same way whichever tool raised it.
package mcpresult

import (
	"github.com/mark3labs/mcp-go/mcp"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/to"
)

// Fail sends a use case's error to the client. It is returned as the handler's
// Go error, not as a tool error result, exactly as the handlers always did.
func Fail(err error) (*mcp.CallToolResult, error) {
	return to.ErrorResult(err)
}

// Render sends a use case's result, or its error, to the client as JSON.
func Render(v any, err error) (*mcp.CallToolResult, error) {
	if err != nil {
		return Fail(err)
	}
	return to.TextResult(v)
}
