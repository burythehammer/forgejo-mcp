// SPDX-License-Identifier: GPL-3.0-or-later

package attachment

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	appattachment "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/attachment"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/attachmentadapter"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/to"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/upload"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// defaultAttachmentUploadTimeout bounds one issue or comment upload, so a stuck
// network path fails with a clear, specific error well inside the caller's
// patience. It is not applied to release assets, which may legitimately be
// much larger; those rely on the HTTP client's own timeout.
const defaultAttachmentUploadTimeout = 45 * time.Second

// attachmentUploadTimeout is a var so tests can shrink it; production never
// reassigns it.
var attachmentUploadTimeout = defaultAttachmentUploadTimeout

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
// port so each request gets the port for its own credential. Each handler is
// the MCP adapter of one use case in app/attachment: read the arguments, run
// the use case, render the result.
type handlers struct{ forge forge.Factory }

// useCases returns the attachment use cases over the request's Forgejo port.
func useCases(ctx context.Context, h handlers) (*appattachment.Service, error) {
	port, err := h.forge.For(ctx)
	if err != nil {
		return nil, err
	}
	return appattachment.New(attachmentadapter.New(port), attachmentUploadTimeout), nil
}

// --- Issue-scoped handlers --------------------------------------------------

func (h handlers) ListIssueAttachmentsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListIssueAttachmentsFn")
	return h.list(ctx, req, appattachment.ScopeIssue, "index")
}

func (h handlers) GetIssueAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetIssueAttachmentFn")
	return h.get(ctx, req, appattachment.ScopeIssue, "index")
}

func (h handlers) DownloadIssueAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DownloadIssueAttachmentFn")
	return h.download(ctx, req, appattachment.ScopeIssue, "index")
}

func (h handlers) CreateIssueAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateIssueAttachmentFn")
	return h.create(ctx, req, appattachment.ScopeIssue, "index")
}

func (h handlers) EditIssueAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called EditIssueAttachmentFn")
	return h.edit(ctx, req, appattachment.ScopeIssue, "index")
}

func (h handlers) DeleteIssueAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteIssueAttachmentFn")
	return h.remove(ctx, req, appattachment.ScopeIssue, "index")
}

// --- Comment-scoped handlers ------------------------------------------------

func (h handlers) ListCommentAttachmentsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListCommentAttachmentsFn")
	return h.list(ctx, req, appattachment.ScopeComment, "comment_id")
}

func (h handlers) GetCommentAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetCommentAttachmentFn")
	return h.get(ctx, req, appattachment.ScopeComment, "comment_id")
}

func (h handlers) DownloadCommentAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DownloadCommentAttachmentFn")
	return h.download(ctx, req, appattachment.ScopeComment, "comment_id")
}

func (h handlers) CreateCommentAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateCommentAttachmentFn")
	return h.create(ctx, req, appattachment.ScopeComment, "comment_id")
}

func (h handlers) EditCommentAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called EditCommentAttachmentFn")
	return h.edit(ctx, req, appattachment.ScopeComment, "comment_id")
}

func (h handlers) DeleteCommentAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteCommentAttachmentFn")
	return h.remove(ctx, req, appattachment.ScopeComment, "comment_id")
}

// --- shared flows -----------------------------------------------------------
//
// Each flow reads the parent argument named by parentArg (index for an issue,
// comment_id for a comment) and attachment_id where a tool takes one.

// number reads a required numeric argument, with the tool's error text.
func number(args map[string]any, name string) (int64, error) {
	v, err := to.Float64(args[name])
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	return int64(v), nil
}

func (h handlers) list(ctx context.Context, req mcp.CallToolRequest, sc appattachment.Scope, parentArg string) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	parent, err := number(args, parentArg)
	if err != nil {
		return to.ErrorResult(err)
	}
	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	out, err := svc.List(ctx, owner, repo, sc, parent)
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(out)
}

func (h handlers) get(ctx context.Context, req mcp.CallToolRequest, sc appattachment.Scope, parentArg string) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	parent, err := number(args, parentArg)
	if err != nil {
		return to.ErrorResult(err)
	}
	id, err := number(args, "attachment_id")
	if err != nil {
		return to.ErrorResult(err)
	}
	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	att, err := svc.Get(ctx, owner, repo, sc, parent, id)
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(att)
}

func (h handlers) download(ctx context.Context, req mcp.CallToolRequest, sc appattachment.Scope, parentArg string) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	parent, err := number(args, parentArg)
	if err != nil {
		return to.ErrorResult(err)
	}
	id, err := number(args, "attachment_id")
	if err != nil {
		return to.ErrorResult(err)
	}
	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	dl, err := svc.Download(ctx, owner, repo, sc, parent, id)
	if err != nil {
		return to.ErrorResult(err)
	}
	if !dl.Result.Inline {
		return to.TextResult(dl.Result)
	}
	// The JSON envelope is the text; the bytes ride along as an embedded blob.
	// Clients that do not know embedded resources still see the JSON.
	textPart := to.SafeJSONMarshal(dl.Result)
	return mcp.NewToolResultResource(textPart, mcp.BlobResourceContents{
		URI:      dl.Result.Attachment.DownloadURL,
		MIMEType: dl.Body.ContentType,
		Blob:     base64.StdEncoding.EncodeToString(dl.Body.Body),
	}), nil
}

func (h handlers) create(ctx context.Context, req mcp.CallToolRequest, sc appattachment.Scope, parentArg string) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	parent, err := number(args, parentArg)
	if err != nil {
		return to.ErrorResult(err)
	}
	mimeType, _ := args["mime_type"].(string)
	reader, filename, err := upload.Open(upload.SourceFromArguments(args))
	if err != nil {
		return to.ErrorResult(err)
	}
	defer reader.Close()

	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	att, err := svc.Create(ctx, owner, repo, sc, parent, appattachment.Upload{Filename: filename, MIMEType: mimeType, Content: reader})
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(att)
}

func (h handlers) edit(ctx context.Context, req mcp.CallToolRequest, sc appattachment.Scope, parentArg string) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	parent, err := number(args, parentArg)
	if err != nil {
		return to.ErrorResult(err)
	}
	id, err := number(args, "attachment_id")
	if err != nil {
		return to.ErrorResult(err)
	}
	name, _ := args["name"].(string)
	if name == "" {
		return to.ErrorResult(fmt.Errorf("name is required"))
	}
	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	att, err := svc.Edit(ctx, owner, repo, sc, parent, id, name)
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(att)
}

func (h handlers) remove(ctx context.Context, req mcp.CallToolRequest, sc appattachment.Scope, parentArg string) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	parent, err := number(args, parentArg)
	if err != nil {
		return to.ErrorResult(err)
	}
	id, err := number(args, "attachment_id")
	if err != nil {
		return to.ErrorResult(err)
	}
	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	if err := svc.Delete(ctx, owner, repo, sc, parent, id); err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(map[string]string{"status": "deleted"})
}
