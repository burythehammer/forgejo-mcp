// SPDX-License-Identifier: GPL-3.0-or-later

package release

import (
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/operation/mcptest"

	"github.com/mark3labs/mcp-go/mcp"
)

// TestArgsMatchSchemas keeps each release tool's argument struct in agreement
// with its input schema: same names, types and required set.
func TestArgsMatchSchemas(t *testing.T) {
	t.Parallel()
	cases := []struct {
		tool mcp.Tool
		args any
	}{
		{ListReleasesTool, listReleasesArgs{}},
		{GetReleaseByIDTool, releaseRefArgs{}},
		{GetReleaseByTagTool, tagRefArgs{}},
		{GetLatestReleaseTool, repoArgs{}},
		{CreateReleaseTool, createReleaseArgs{}},
		{EditReleaseTool, editReleaseArgs{}},
		{DeleteReleaseTool, releaseRefArgs{}},
		{DeleteReleaseByTagTool, tagRefArgs{}},
		{ListReleaseAttachmentsTool, listAttachmentsArgs{}},
		{GetReleaseAttachmentTool, attachmentRefArgs{}},
		{DownloadReleaseAttachmentTool, attachmentRefArgs{}},
		{CreateReleaseAttachmentTool, createAttachmentArgs{}},
		{EditReleaseAttachmentTool, editAttachmentArgs{}},
		{DeleteReleaseAttachmentTool, attachmentRefArgs{}},
	}
	for _, c := range cases {
		t.Run(c.tool.Name, func(t *testing.T) { mcptest.ArgsMatchSchema(t, c.tool, c.args) })
	}
}
