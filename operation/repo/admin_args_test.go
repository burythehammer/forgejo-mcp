// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/operation/mcptest"

	"github.com/mark3labs/mcp-go/mcp"
)

// Each argument struct must agree with its tool's input schema: the same
// property names, compatible types and the same required set.
func TestAdminArgStructsMatchTheirSchemas(t *testing.T) {
	t.Parallel()
	cases := []struct {
		tool mcp.Tool
		args any
	}{
		{CreateRepoTool, createRepoArgs{}},
		{ForkRepoTool, forkRepoArgs{}},
		{ListMyReposTool, listMyReposArgs{}},
		{GetRepoTool, getRepoArgs{}},
		{EditRepoTool, editRepoArgs{}},
		{ListRepoTopicsTool, listRepoTopicsArgs{}},
		{SetRepoTopicsTool, setRepoTopicsArgs{}},
		{AddRepoTopicTool, addRepoTopicArgs{}},
		{DeleteRepoTopicTool, addRepoTopicArgs{}},
		{GetCommitStatusesTool, getCommitStatusesArgs{}},
	}
	for _, c := range cases {
		t.Run(c.tool.Name, func(t *testing.T) {
			t.Parallel()
			mcptest.ArgsMatchSchema(t, c.tool, c.args)
		})
	}
}
