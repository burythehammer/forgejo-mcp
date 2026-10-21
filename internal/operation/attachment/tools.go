// SPDX-License-Identifier: GPL-3.0-or-later

// Package attachment registers the MCP tools for issue and issue-comment
// attachments. Each tool is an MCP adapter over a use case in internal/app/attachment:
// this file holds the tool definitions, handlers.go the handlers.
package attachment

import (
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/params"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	// Issue-scoped tool names
	ListIssueAttachmentsToolName    = "list_issue_attachments"
	GetIssueAttachmentToolName      = "get_issue_attachment"
	DownloadIssueAttachmentToolName = "download_issue_attachment"
	CreateIssueAttachmentToolName   = "create_issue_attachment"
	EditIssueAttachmentToolName     = "edit_issue_attachment"
	DeleteIssueAttachmentToolName   = "delete_issue_attachment"

	// Comment-scoped tool names
	ListCommentAttachmentsToolName    = "list_comment_attachments"
	GetCommentAttachmentToolName      = "get_comment_attachment"
	DownloadCommentAttachmentToolName = "download_comment_attachment"
	CreateCommentAttachmentToolName   = "create_comment_attachment"
	EditCommentAttachmentToolName     = "edit_comment_attachment"
	DeleteCommentAttachmentToolName   = "delete_comment_attachment"
)

var (
	ListIssueAttachmentsTool = mcp.NewTool(
		ListIssueAttachmentsToolName,
		mcp.WithDescription("List attachments on an issue or pull request."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.Index)),
	)

	GetIssueAttachmentTool = mcp.NewTool(
		GetIssueAttachmentToolName,
		mcp.WithDescription("Get metadata for a single issue/PR attachment."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.Index)),
		mcp.WithNumber("attachment_id", mcp.Required(), mcp.Description(params.AttachmentID)),
	)

	DownloadIssueAttachmentTool = mcp.NewTool(
		DownloadIssueAttachmentToolName,
		mcp.WithDescription("Download an issue/PR attachment. Files at or above the inline cap return metadata + browser_download_url only; the caller is expected to fetch that URL with the same auth token."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.Index)),
		mcp.WithNumber("attachment_id", mcp.Required(), mcp.Description(params.AttachmentID)),
	)

	CreateIssueAttachmentTool = mcp.NewTool(
		CreateIssueAttachmentToolName,
		mcp.WithDescription("Upload a new attachment to an issue or pull request from exactly one of base64 content or a file path on the forgejo-mcp host."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.Index)),
		mcp.WithString("content", mcp.Description(params.AttachmentContent)),
		mcp.WithString("file_path", mcp.Description(params.AttachmentFilePath)),
		mcp.WithString("filename", mcp.Description(params.AttachmentFilename)),
		mcp.WithString("mime_type", mcp.Description(params.AttachmentMIME)),
	)

	EditIssueAttachmentTool = mcp.NewTool(
		EditIssueAttachmentToolName,
		mcp.WithDescription("Rename an issue/PR attachment."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.Index)),
		mcp.WithNumber("attachment_id", mcp.Required(), mcp.Description(params.AttachmentID)),
		mcp.WithString("name", mcp.Required(), mcp.Description(params.AttachmentName)),
	)

	DeleteIssueAttachmentTool = mcp.NewTool(
		DeleteIssueAttachmentToolName,
		mcp.WithDescription("Delete an issue/PR attachment."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(params.Index)),
		mcp.WithNumber("attachment_id", mcp.Required(), mcp.Description(params.AttachmentID)),
	)

	ListCommentAttachmentsTool = mcp.NewTool(
		ListCommentAttachmentsToolName,
		mcp.WithDescription("List attachments on an issue/PR comment."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("comment_id", mcp.Required(), mcp.Description(params.CommentID)),
	)

	GetCommentAttachmentTool = mcp.NewTool(
		GetCommentAttachmentToolName,
		mcp.WithDescription("Get metadata for a single comment attachment."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("comment_id", mcp.Required(), mcp.Description(params.CommentID)),
		mcp.WithNumber("attachment_id", mcp.Required(), mcp.Description(params.AttachmentID)),
	)

	DownloadCommentAttachmentTool = mcp.NewTool(
		DownloadCommentAttachmentToolName,
		mcp.WithDescription("Download a comment attachment. Files at or above the inline cap return metadata + browser_download_url only; the caller is expected to fetch that URL with the same auth token."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("comment_id", mcp.Required(), mcp.Description(params.CommentID)),
		mcp.WithNumber("attachment_id", mcp.Required(), mcp.Description(params.AttachmentID)),
	)

	CreateCommentAttachmentTool = mcp.NewTool(
		CreateCommentAttachmentToolName,
		mcp.WithDescription("Upload a new attachment to an issue/PR comment from exactly one of base64 content or a file path on the forgejo-mcp host."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("comment_id", mcp.Required(), mcp.Description(params.CommentID)),
		mcp.WithString("content", mcp.Description(params.AttachmentContent)),
		mcp.WithString("file_path", mcp.Description(params.AttachmentFilePath)),
		mcp.WithString("filename", mcp.Description(params.AttachmentFilename)),
		mcp.WithString("mime_type", mcp.Description(params.AttachmentMIME)),
	)

	EditCommentAttachmentTool = mcp.NewTool(
		EditCommentAttachmentToolName,
		mcp.WithDescription("Rename a comment attachment."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("comment_id", mcp.Required(), mcp.Description(params.CommentID)),
		mcp.WithNumber("attachment_id", mcp.Required(), mcp.Description(params.AttachmentID)),
		mcp.WithString("name", mcp.Required(), mcp.Description(params.AttachmentName)),
	)

	DeleteCommentAttachmentTool = mcp.NewTool(
		DeleteCommentAttachmentToolName,
		mcp.WithDescription("Delete a comment attachment."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("comment_id", mcp.Required(), mcp.Description(params.CommentID)),
		mcp.WithNumber("attachment_id", mcp.Required(), mcp.Description(params.AttachmentID)),
	)
)
