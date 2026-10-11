// Package attachment registers MCP tools for issue and issue-comment
// attachments. It reaches Forgejo through the forge.Attachments port; the adapter uses raw
// HTTP because forgejo-sdk/v3 has no methods for these endpoints (see
// docs/plans/issue-attachments.md).
package attachment

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/params"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/to"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/upload"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Upload timeout. Multiple agents reported
// create_*_attachment (issue and comment attachments) failing with "illegal
// base64 data" at a variable byte
// offset (~1.5KB-4.9KB observed) and, once, hanging a caller for ~30
// minutes. A size-sweep repro against the real stdio transport (see
// test/transport/sweep_test.go) found no corruption or hang in this server
// for payloads from 1KB to 100KB — the root cause is upstream of this
// process (the calling MCP client/harness), not in this server's read,
// decode, or upload path. This guard exists regardless, as defense in
// depth: it bounds worst-case wall-clock time for a stuck network path so a
// broken upload always fails fast with a clear, actionable error instead of
// hanging.
//
// This timeout is scoped to create_issue_attachment / create_comment_attachment
// only — it is NOT applied to create_release_attachment. Release assets can
// legitimately be much larger than an issue/comment attachment (build
// artifacts, archives, container images) and may need more than 45s of
// upload headroom; create_release_attachment relies on the underlying HTTP
// client's own (longer) timeout instead. The base64 `content` size cap and
// received-length decode diagnostic, by contrast, live in pkg/upload.Open
// and apply uniformly to all three attachment-creating tools, since
// create_release_attachment accepts base64 `content` exactly like the
// issue/comment tools do (see operation/release/release.go).
const (
	// defaultAttachmentUploadTimeout bounds the multipart upload call so a
	// stuck network path fails with a clear, specific error well inside the
	// caller's own patience, instead of running to the underlying HTTP
	// client's 60s timeout (or beyond, if something upstream never returns
	// control at all).
	defaultAttachmentUploadTimeout = 45 * time.Second
)

// attachmentUploadTimeout is a var (not const) so tests can shrink it to
// exercise the timeout path without a real 45s wait; production code never
// reassigns it, so it always behaves as the 45s constant above.
var attachmentUploadTimeout = defaultAttachmentUploadTimeout

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

// downloadResult is the shape returned by download_*_attachment when bytes
// are inlined. The Blob field carries the embedded resource separately;
// this struct is only used for the metadata-only path.
type downloadResult struct {
	Attachment    *forgejo_sdk.Attachment `json:"attachment"`
	Inline        bool                    `json:"inline"`
	Reason        string                  `json:"reason,omitempty"`
	BytesIncluded int64                   `json:"bytes_included,omitempty"`
}

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

// RegisterTool registers all 12 attachment tools with the MCP server. Each call
// takes its Forgejo port from f, so a request is served with the credential
// that request carries.
func RegisterTool(s *server.MCPServer, f forge.Factory) {
	h := handlers{forge: f}
	s.AddTool(ListIssueAttachmentsTool, h.ListIssueAttachmentsFn)
	s.AddTool(GetIssueAttachmentTool, h.GetIssueAttachmentFn)
	s.AddTool(DownloadIssueAttachmentTool, h.DownloadIssueAttachmentFn)
	s.AddTool(CreateIssueAttachmentTool, h.CreateIssueAttachmentFn)
	s.AddTool(EditIssueAttachmentTool, h.EditIssueAttachmentFn)
	s.AddTool(DeleteIssueAttachmentTool, h.DeleteIssueAttachmentFn)

	s.AddTool(ListCommentAttachmentsTool, h.ListCommentAttachmentsFn)
	s.AddTool(GetCommentAttachmentTool, h.GetCommentAttachmentFn)
	s.AddTool(DownloadCommentAttachmentTool, h.DownloadCommentAttachmentFn)
	s.AddTool(CreateCommentAttachmentTool, h.CreateCommentAttachmentFn)
	s.AddTool(EditCommentAttachmentTool, h.EditCommentAttachmentFn)
	s.AddTool(DeleteCommentAttachmentTool, h.DeleteCommentAttachmentFn)
}

// handlers serve the attachment tools. They hold the factory rather than a
// port so each request gets the port for its own credential.
type handlers struct{ forge forge.Factory }

// --- Issue-scoped handlers --------------------------------------------------

func (h handlers) ListIssueAttachmentsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListIssueAttachmentsFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, err := to.Float64(req.GetArguments()["index"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("index: %w", err))
	}

	port, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	out, err := port.ListIssueAttachments(ctx, owner, repo, int64(index))
	return listResult(out, err, "list issue attachments err")
}

// listResult renders an attachment list. Forgejo answers 404 for a parent with
// no attachments as often as for a missing one, so a 404 is an empty list.
func listResult(out []*forgejo_sdk.Attachment, err error, what string) (*mcp.CallToolResult, error) {
	if err != nil && !forge.IsNotFound(err) {
		return to.ErrorResult(fmt.Errorf("%s: %w", what, err))
	}
	if out == nil {
		out = []*forgejo_sdk.Attachment{}
	}
	return to.TextResult(out)
}

func (h handlers) GetIssueAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetIssueAttachmentFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, err := to.Float64(req.GetArguments()["index"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("index: %w", err))
	}
	aid, err := to.Float64(req.GetArguments()["attachment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("attachment_id: %w", err))
	}

	port, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	att, err := port.GetIssueAttachment(ctx, owner, repo, int64(index), int64(aid))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get issue attachment err: %w", err))
	}
	return to.TextResult(att)
}

func (h handlers) DownloadIssueAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DownloadIssueAttachmentFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, err := to.Float64(req.GetArguments()["index"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("index: %w", err))
	}
	aid, err := to.Float64(req.GetArguments()["attachment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("attachment_id: %w", err))
	}

	port, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	att, err := port.GetIssueAttachment(ctx, owner, repo, int64(index), int64(aid))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("download issue attachment (metadata) err: %w", err))
	}
	return downloadResultFor(ctx, port, att)
}

func (h handlers) CreateIssueAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateIssueAttachmentFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	index, err := to.Float64(args["index"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("index: %w", err))
	}
	mimeType, _ := args["mime_type"].(string)
	reader, filename, err := upload.Open(upload.SourceFromArguments(args))
	if err != nil {
		return to.ErrorResult(err)
	}
	defer reader.Close()

	port, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	uploadCtx, cancel := context.WithTimeout(ctx, attachmentUploadTimeout)
	defer cancel()

	att, err := port.CreateIssueAttachment(uploadCtx, owner, repo, int64(index), forge.Upload{Filename: filename, MIMEType: mimeType, Content: reader})
	if err != nil {
		return to.ErrorResult(fmt.Errorf("create issue attachment err: %w", wrapUploadTimeout(ctx, uploadCtx, err)))
	}
	return to.TextResult(att)
}

func (h handlers) EditIssueAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called EditIssueAttachmentFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, err := to.Float64(req.GetArguments()["index"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("index: %w", err))
	}
	aid, err := to.Float64(req.GetArguments()["attachment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("attachment_id: %w", err))
	}
	name, _ := req.GetArguments()["name"].(string)
	if name == "" {
		return to.ErrorResult(fmt.Errorf("name is required"))
	}

	port, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	att, err := port.EditIssueAttachment(ctx, owner, repo, int64(index), int64(aid), name)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("edit issue attachment err: %w", err))
	}
	return to.TextResult(att)
}

func (h handlers) DeleteIssueAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteIssueAttachmentFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, err := to.Float64(req.GetArguments()["index"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("index: %w", err))
	}
	aid, err := to.Float64(req.GetArguments()["attachment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("attachment_id: %w", err))
	}

	port, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	if err := port.DeleteIssueAttachment(ctx, owner, repo, int64(index), int64(aid)); err != nil {
		return to.ErrorResult(fmt.Errorf("delete issue attachment err: %w", err))
	}
	return to.TextResult(map[string]string{"status": "deleted"})
}

// --- Comment-scoped handlers ------------------------------------------------

func (h handlers) ListCommentAttachmentsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListCommentAttachmentsFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	cid, err := to.Float64(req.GetArguments()["comment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("comment_id: %w", err))
	}

	port, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	out, err := port.ListCommentAttachments(ctx, owner, repo, int64(cid))
	return listResult(out, err, "list comment attachments err")
}

func (h handlers) GetCommentAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetCommentAttachmentFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	cid, err := to.Float64(req.GetArguments()["comment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("comment_id: %w", err))
	}
	aid, err := to.Float64(req.GetArguments()["attachment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("attachment_id: %w", err))
	}

	port, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	att, err := port.GetCommentAttachment(ctx, owner, repo, int64(cid), int64(aid))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get comment attachment err: %w", err))
	}
	return to.TextResult(att)
}

func (h handlers) DownloadCommentAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DownloadCommentAttachmentFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	cid, err := to.Float64(req.GetArguments()["comment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("comment_id: %w", err))
	}
	aid, err := to.Float64(req.GetArguments()["attachment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("attachment_id: %w", err))
	}

	port, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	att, err := port.GetCommentAttachment(ctx, owner, repo, int64(cid), int64(aid))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("download comment attachment (metadata) err: %w", err))
	}
	return downloadResultFor(ctx, port, att)
}

func (h handlers) CreateCommentAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateCommentAttachmentFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	cid, err := to.Float64(args["comment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("comment_id: %w", err))
	}
	mimeType, _ := args["mime_type"].(string)
	reader, filename, err := upload.Open(upload.SourceFromArguments(args))
	if err != nil {
		return to.ErrorResult(err)
	}
	defer reader.Close()

	port, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	uploadCtx, cancel := context.WithTimeout(ctx, attachmentUploadTimeout)
	defer cancel()

	att, err := port.CreateCommentAttachment(uploadCtx, owner, repo, int64(cid), forge.Upload{Filename: filename, MIMEType: mimeType, Content: reader})
	if err != nil {
		return to.ErrorResult(fmt.Errorf("create comment attachment err: %w", wrapUploadTimeout(ctx, uploadCtx, err)))
	}
	return to.TextResult(att)
}

func (h handlers) EditCommentAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called EditCommentAttachmentFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	cid, err := to.Float64(req.GetArguments()["comment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("comment_id: %w", err))
	}
	aid, err := to.Float64(req.GetArguments()["attachment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("attachment_id: %w", err))
	}
	name, _ := req.GetArguments()["name"].(string)
	if name == "" {
		return to.ErrorResult(fmt.Errorf("name is required"))
	}

	port, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	att, err := port.EditCommentAttachment(ctx, owner, repo, int64(cid), int64(aid), name)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("edit comment attachment err: %w", err))
	}
	return to.TextResult(att)
}

func (h handlers) DeleteCommentAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteCommentAttachmentFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	cid, err := to.Float64(req.GetArguments()["comment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("comment_id: %w", err))
	}
	aid, err := to.Float64(req.GetArguments()["attachment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("attachment_id: %w", err))
	}

	port, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	if err := port.DeleteCommentAttachment(ctx, owner, repo, int64(cid), int64(aid)); err != nil {
		return to.ErrorResult(fmt.Errorf("delete comment attachment err: %w", err))
	}
	return to.TextResult(map[string]string{"status": "deleted"})
}

// wrapUploadTimeout annotates err with a clear, specific message when the
// upload's own context deadline (attachmentUploadTimeout) is what ended the
// call, so a caller sees "upload timed out after 45s" instead of a bare
// "context deadline exceeded" buried inside an HTTP transport error.
//
// parent is the caller's original context (before attachmentUploadTimeout
// was applied) and uploadCtx is the child context actually passed to the
// upload call. Both must be checked: if parent's own deadline is what
// elapsed — e.g. an overall request timeout imposed by the MCP host, unaware
// of and unrelated to attachmentUploadTimeout — parent.Err() is already
// DeadlineExceeded by the time the upload call returns (context.WithTimeout
// derives uploadCtx from parent, so a parent-caused expiry propagates the
// same error down), and the message must not claim "upload timed out after
// 45s" for a deadline this function's own budget had nothing to do with.
func wrapUploadTimeout(parent, uploadCtx context.Context, err error) error {
	if !errors.Is(uploadCtx.Err(), context.DeadlineExceeded) {
		return err
	}
	if errors.Is(parent.Err(), context.DeadlineExceeded) {
		return err
	}
	return fmt.Errorf("upload timed out after %s: %w", attachmentUploadTimeout, err)
}

// --- shared helpers ---------------------------------------------------------

// downloadResultFor fetches inline bytes when the attachment is under the
// inline cap; otherwise returns metadata + URL only and instructs the caller
// to fetch via curl. Always includes browser_download_url.
func downloadResultFor(ctx context.Context, port forge.Attachments, att *forgejo_sdk.Attachment) (*mcp.CallToolResult, error) {
	res := &downloadResult{Attachment: att}

	if att.Size >= forgejo.MaxInlineDownloadBytes {
		res.Inline = false
		res.Reason = fmt.Sprintf("size %d bytes >= inline cap %d; fetch browser_download_url with Authorization: token <TOKEN>", att.Size, forgejo.MaxInlineDownloadBytes)
		return to.TextResult(res)
	}

	dl, err := port.DownloadAttachment(ctx, att.DownloadURL)
	if err != nil {
		// Defensive: if the size advertised was under cap but the actual bytes
		// blow past it, treat that the same as "too big" rather than failing.
		if errors.Is(err, forge.ErrPayloadTooLarge) {
			res.Inline = false
			res.Reason = "body exceeded inline cap during fetch; fetch browser_download_url with Authorization: token <TOKEN>"
			return to.TextResult(res)
		}
		return to.ErrorResult(fmt.Errorf("download body err: %w", err))
	}
	res.Inline = true
	res.BytesIncluded = int64(len(dl.Body))

	uri := att.DownloadURL
	mimeType := dl.ContentType
	encoded := base64.StdEncoding.EncodeToString(dl.Body)

	// NewToolResultResource gives us a CallToolResult containing both a
	// text content (the metadata-as-JSON) and an embedded BlobResourceContents.
	// MCP clients that don't know about embedded resources still see the JSON.
	textPart := to.SafeJSONMarshal(res)
	return mcp.NewToolResultResource(textPart, mcp.BlobResourceContents{
		URI:      uri,
		MIMEType: mimeType,
		Blob:     encoded,
	}), nil
}
