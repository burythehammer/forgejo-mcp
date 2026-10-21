// SPDX-License-Identifier: GPL-3.0-or-later

package branchprotection

import (
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcptest"
)

// TestArgsMatchSchema keeps each tool's argument struct in agreement with its
// input schema: same names, types and required set.
func TestArgsMatchSchema(t *testing.T) {
	t.Parallel()
	mcptest.ArgsMatchSchema(t, ListBranchProtectionsTool, listArgs{})
	mcptest.ArgsMatchSchema(t, GetBranchProtectionTool, ruleArgs{})
	mcptest.ArgsMatchSchema(t, CreateBranchProtectionTool, createArgs{})
	mcptest.ArgsMatchSchema(t, EditBranchProtectionTool, editArgs{})
	mcptest.ArgsMatchSchema(t, DeleteBranchProtectionTool, ruleArgs{})
}
