// SPDX-License-Identifier: GPL-3.0-or-later

package wiki

import (
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/mcptest"
)

// TestArgsMatchSchema keeps each tool's argument struct in agreement with its
// input schema: same names, types and required set.
func TestArgsMatchSchema(t *testing.T) {
	t.Parallel()
	mcptest.ArgsMatchSchema(t, ListWikiPagesTool, listArgs{})
	mcptest.ArgsMatchSchema(t, GetWikiPageTool, getArgs{})
	mcptest.ArgsMatchSchema(t, GetWikiRevisionsTool, revisionsArgs{})
	mcptest.ArgsMatchSchema(t, CreateWikiPageTool, createArgs{})
	mcptest.ArgsMatchSchema(t, UpdateWikiPageTool, updateArgs{})
	mcptest.ArgsMatchSchema(t, DeleteWikiPageTool, pageArgs{})
}
