// SPDX-License-Identifier: GPL-3.0-or-later

package hook

import (
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/mcptest"
)

// TestArgsMatchSchema keeps each tool's argument struct in agreement with its
// input schema: same names, types and required set.
func TestArgsMatchSchema(t *testing.T) {
	t.Parallel()
	mcptest.ArgsMatchSchema(t, ListRepoHooksTool, listArgs{})
	mcptest.ArgsMatchSchema(t, GetRepoHookTool, hookRefArgs{})
	mcptest.ArgsMatchSchema(t, CreateRepoHookTool, createArgs{})
	mcptest.ArgsMatchSchema(t, EditRepoHookTool, editArgs{})
	mcptest.ArgsMatchSchema(t, DeleteRepoHookTool, hookRefArgs{})
	mcptest.ArgsMatchSchema(t, TestRepoHookTool, hookRefArgs{})
}
