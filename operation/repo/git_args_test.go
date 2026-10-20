// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/operation/mcptest"

	"github.com/mark3labs/mcp-go/mcp"
)

// The argument structs of the branch, commit, contents and file tools agree
// with their tool definitions. change_files reads its raw arguments and has no
// struct to check.
func TestGitArgsMatchSchemas(t *testing.T) {
	t.Parallel()
	cases := []struct {
		tool mcp.Tool
		args any
	}{
		{CreateBranchTool, createBranchArgs{}},
		{DeleteBranchTool, deleteBranchArgs{}},
		{ListBranchesTool, listBranchesArgs{}},
		{ListRepoCommitsTool, listRepoCommitsArgs{}},
		{ListRepoContentsTool, listRepoContentsArgs{}},
		{GetRepoTreeTool, getRepoTreeArgs{}},
		{GetFileContentTool, getFileContentArgs{}},
		{CreateFileTool, createFileArgs{}},
		{UpdateFileTool, updateFileArgs{}},
		{DeleteFileTool, deleteFileArgs{}},
	}
	for _, c := range cases {
		t.Run(c.tool.Name, func(t *testing.T) { mcptest.ArgsMatchSchema(t, c.tool, c.args) })
	}
}
