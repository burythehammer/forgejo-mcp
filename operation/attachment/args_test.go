// SPDX-License-Identifier: GPL-3.0-or-later

package attachment

import (
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/mcptest"

	"github.com/mark3labs/mcp-go/mcp"
)

// TestArgsMatchSchemas keeps each attachment tool's argument struct in agreement
// with its input schema: same names, types and required set.
func TestArgsMatchSchemas(t *testing.T) {
	t.Parallel()
	cases := []struct {
		tool mcp.Tool
		args any
	}{
		{ListIssueAttachmentsTool, issueRefArgs{}},
		{GetIssueAttachmentTool, issueAttachmentArgs{}},
		{DownloadIssueAttachmentTool, issueAttachmentArgs{}},
		{CreateIssueAttachmentTool, issueCreateArgs{}},
		{EditIssueAttachmentTool, issueEditArgs{}},
		{DeleteIssueAttachmentTool, issueAttachmentArgs{}},
		{ListCommentAttachmentsTool, commentRefArgs{}},
		{GetCommentAttachmentTool, commentAttachmentArgs{}},
		{DownloadCommentAttachmentTool, commentAttachmentArgs{}},
		{CreateCommentAttachmentTool, commentCreateArgs{}},
		{EditCommentAttachmentTool, commentEditArgs{}},
		{DeleteCommentAttachmentTool, commentAttachmentArgs{}},
	}
	for _, c := range cases {
		t.Run(c.tool.Name, func(t *testing.T) { mcptest.ArgsMatchSchema(t, c.tool, c.args) })
	}
}
