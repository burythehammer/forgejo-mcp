// SPDX-License-Identifier: GPL-3.0-or-later

// Package release registers the MCP tools for Forgejo releases and their
// attachments. Each tool is an MCP adapter over a use case in app/release: this
// file holds the tool definitions, handlers.go the handlers.
package release

import (
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/operation/params"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	ListReleasesToolName       = "list_releases"
	GetReleaseByIDToolName     = "get_release_by_id"
	GetReleaseByTagToolName    = "get_release_by_tag"
	GetLatestReleaseToolName   = "get_latest_release"
	CreateReleaseToolName      = "create_release"
	EditReleaseToolName        = "edit_release"
	DeleteReleaseToolName      = "delete_release"
	DeleteReleaseByTagToolName = "delete_release_by_tag"

	ListReleaseAttachmentsToolName    = "list_release_attachments"
	GetReleaseAttachmentToolName      = "get_release_attachment"
	DownloadReleaseAttachmentToolName = "download_release_attachment"
	CreateReleaseAttachmentToolName   = "create_release_attachment"
	EditReleaseAttachmentToolName     = "edit_release_attachment"
	DeleteReleaseAttachmentToolName   = "delete_release_attachment"

	// State filter values for list_releases.
	stateAll        = "all"
	stateDraft      = "draft"
	statePrerelease = "prerelease"
	statePublished  = "published"
)

var (
	ListReleasesTool = mcp.NewTool(
		ListReleasesToolName,
		mcp.WithDescription("List releases for a repository. Returns a response envelope {releases, page, limit, count, has_next}. has_next true means a further page may exist; re-issue the call with page incremented to fetch it. The state filter is applied client-side after pagination, so count may be smaller than limit even when more matches exist on later pages."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(20)),
		mcp.WithString("state", mcp.Description(params.ReleaseState), mcp.DefaultString(stateAll)),
	)

	GetReleaseByIDTool = mcp.NewTool(
		GetReleaseByIDToolName,
		mcp.WithDescription("Get a release by numeric ID."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("release_id", mcp.Required(), mcp.Description(params.ReleaseID)),
	)

	GetReleaseByTagTool = mcp.NewTool(
		GetReleaseByTagToolName,
		mcp.WithDescription("Get a release by tag name."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("tag", mcp.Required(), mcp.Description(params.ReleaseTag)),
	)

	GetLatestReleaseTool = mcp.NewTool(
		GetLatestReleaseToolName,
		mcp.WithDescription("Get the latest non-draft, non-prerelease release."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
	)

	CreateReleaseTool = mcp.NewTool(
		CreateReleaseToolName,
		mcp.WithDescription("Create a release. If the tag does not yet exist, pass target_commitish so Forgejo creates the tag."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("tag_name", mcp.Required(), mcp.Description(params.ReleaseTagName)),
		mcp.WithString("target_commitish", mcp.Description(params.ReleaseTargetCommitish)),
		mcp.WithString("name", mcp.Description(params.Title)),
		mcp.WithString("body", mcp.Description(params.Body)),
		mcp.WithBoolean("draft", mcp.Description(params.ReleaseDraft)),
		mcp.WithBoolean("prerelease", mcp.Description(params.ReleasePrerelease)),
	)

	EditReleaseTool = mcp.NewTool(
		EditReleaseToolName,
		mcp.WithDescription("Edit an existing release. Only fields supplied by the caller are sent to Forgejo."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("release_id", mcp.Required(), mcp.Description(params.ReleaseID)),
		mcp.WithString("tag_name", mcp.Description(params.ReleaseTagName)),
		mcp.WithString("target_commitish", mcp.Description(params.ReleaseTargetCommitish)),
		mcp.WithString("name", mcp.Description(params.Title)),
		mcp.WithString("body", mcp.Description(params.Body)),
		mcp.WithBoolean("draft", mcp.Description(params.ReleaseDraft)),
		mcp.WithBoolean("prerelease", mcp.Description(params.ReleasePrerelease)),
	)

	DeleteReleaseTool = mcp.NewTool(
		DeleteReleaseToolName,
		mcp.WithDescription("Delete a release by numeric ID. Destructive."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("release_id", mcp.Required(), mcp.Description(params.ReleaseID)),
	)

	DeleteReleaseByTagTool = mcp.NewTool(
		DeleteReleaseByTagToolName,
		mcp.WithDescription("Delete a release by tag name. Destructive — verify tag before calling."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("tag", mcp.Required(), mcp.Description(params.ReleaseTag)),
	)

	ListReleaseAttachmentsTool = mcp.NewTool(
		ListReleaseAttachmentsToolName,
		mcp.WithDescription("List attachments on a release. Returns a response envelope {attachments, page, limit, count, has_next}. has_next true means a further page may exist; re-issue the call with page incremented to fetch it. The Forgejo API does not paginate this endpoint server-side, so the response is fetched in full and then sliced client-side; large attachment sets are still fully transferred from Forgejo before slicing."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("release_id", mcp.Required(), mcp.Description(params.ReleaseID)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(20)),
	)

	GetReleaseAttachmentTool = mcp.NewTool(
		GetReleaseAttachmentToolName,
		mcp.WithDescription("Get metadata for a single release attachment."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("release_id", mcp.Required(), mcp.Description(params.ReleaseID)),
		mcp.WithNumber("attachment_id", mcp.Required(), mcp.Description(params.AttachmentID)),
	)

	DownloadReleaseAttachmentTool = mcp.NewTool(
		DownloadReleaseAttachmentToolName,
		mcp.WithDescription("Download a release attachment. Files at or above the inline cap return metadata + browser_download_url only; the caller is expected to fetch that URL with the same auth token."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("release_id", mcp.Required(), mcp.Description(params.ReleaseID)),
		mcp.WithNumber("attachment_id", mcp.Required(), mcp.Description(params.AttachmentID)),
	)

	CreateReleaseAttachmentTool = mcp.NewTool(
		CreateReleaseAttachmentToolName,
		mcp.WithDescription("Upload an attachment to a release from exactly one of base64 content or a file path on the forgejo-mcp host."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("release_id", mcp.Required(), mcp.Description(params.ReleaseID)),
		mcp.WithString("content", mcp.Description(params.AttachmentContent)),
		mcp.WithString("file_path", mcp.Description(params.AttachmentFilePath)),
		mcp.WithString("filename", mcp.Description(params.AttachmentFilename)),
		mcp.WithString("mime_type", mcp.Description(params.AttachmentMIME)),
	)

	EditReleaseAttachmentTool = mcp.NewTool(
		EditReleaseAttachmentToolName,
		mcp.WithDescription("Rename a release attachment."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("release_id", mcp.Required(), mcp.Description(params.ReleaseID)),
		mcp.WithNumber("attachment_id", mcp.Required(), mcp.Description(params.AttachmentID)),
		mcp.WithString("name", mcp.Required(), mcp.Description(params.AttachmentName)),
	)

	DeleteReleaseAttachmentTool = mcp.NewTool(
		DeleteReleaseAttachmentToolName,
		mcp.WithDescription("Delete a release attachment. Destructive."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("release_id", mcp.Required(), mcp.Description(params.ReleaseID)),
		mcp.WithNumber("attachment_id", mcp.Required(), mcp.Description(params.AttachmentID)),
	)
)
