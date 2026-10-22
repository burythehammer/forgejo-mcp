// SPDX-License-Identifier: GPL-3.0-or-later

package attachment

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	appattachment "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/attachment"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/attachmentadapter"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/log"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/decode"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/to"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/upload"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// defaultAttachmentUploadTimeout bounds one issue or comment upload, so a stuck
// network path fails with a clear, specific error well inside the caller's
// patience. It is not applied to release assets, which may legitimately be
// much larger; those are bounded by the upload client's uploadRequestTimeout.
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
// the MCP adapter of one use case in internal/app/attachment: read the arguments, run
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

// Argument structs. Issue tools take `index` as their parent, comment tools
// `comment_id`; each shape has its own struct so the tool's schema is checked
// by mcptest.ArgsMatchSchema. `json` names the MCP argument, `required:"true"`
// marks the required ones.
//
// Required numbers are kept as `any` (schema:"number") and read by number(),
// so a missing or wrongly typed one fails with its "<name>: <cause>" text,
// which decode.Lax would otherwise hide as zero.

type issueRefArgs struct {
	Owner string `json:"owner" required:"true"`
	Repo  string `json:"repo" required:"true"`
	Index any    `json:"index" required:"true" schema:"number"`
}

type commentRefArgs struct {
	Owner     string `json:"owner" required:"true"`
	Repo      string `json:"repo" required:"true"`
	CommentID any    `json:"comment_id" required:"true" schema:"number"`
}

type issueAttachmentArgs struct {
	Owner        string `json:"owner" required:"true"`
	Repo         string `json:"repo" required:"true"`
	Index        any    `json:"index" required:"true" schema:"number"`
	AttachmentID any    `json:"attachment_id" required:"true" schema:"number"`
}

type commentAttachmentArgs struct {
	Owner        string `json:"owner" required:"true"`
	Repo         string `json:"repo" required:"true"`
	CommentID    any    `json:"comment_id" required:"true" schema:"number"`
	AttachmentID any    `json:"attachment_id" required:"true" schema:"number"`
}

// issueCreateArgs and commentCreateArgs declare content and file_path so the
// schema check covers them, but the upload source is read from the raw
// arguments by upload.SourceFromArguments: it must tell an explicit null or
// wrongly typed value (present, empty) from an absent one, which decode.Lax
// cannot.
type issueCreateArgs struct {
	Owner    string `json:"owner" required:"true"`
	Repo     string `json:"repo" required:"true"`
	Index    any    `json:"index" required:"true" schema:"number"`
	Content  any    `json:"content" schema:"string"`
	FilePath any    `json:"file_path" schema:"string"`
	Filename string `json:"filename"`
	MimeType string `json:"mime_type"`
}

type commentCreateArgs struct {
	Owner     string `json:"owner" required:"true"`
	Repo      string `json:"repo" required:"true"`
	CommentID any    `json:"comment_id" required:"true" schema:"number"`
	Content   any    `json:"content" schema:"string"`
	FilePath  any    `json:"file_path" schema:"string"`
	Filename  string `json:"filename"`
	MimeType  string `json:"mime_type"`
}

type issueEditArgs struct {
	Owner        string `json:"owner" required:"true"`
	Repo         string `json:"repo" required:"true"`
	Index        any    `json:"index" required:"true" schema:"number"`
	AttachmentID any    `json:"attachment_id" required:"true" schema:"number"`
	Name         string `json:"name" required:"true"`
}

type commentEditArgs struct {
	Owner        string `json:"owner" required:"true"`
	Repo         string `json:"repo" required:"true"`
	CommentID    any    `json:"comment_id" required:"true" schema:"number"`
	AttachmentID any    `json:"attachment_id" required:"true" schema:"number"`
	Name         string `json:"name" required:"true"`
}

// number reads a required numeric argument, with the tool's error text.
func number(v any, name string) (int64, error) {
	f, err := to.Float64(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	return int64(f), nil
}

// --- Issue-scoped handlers --------------------------------------------------

func (h handlers) ListIssueAttachmentsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListIssueAttachmentsFn")
	var a issueRefArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	return h.list(ctx, a.Owner, a.Repo, appattachment.ScopeIssue, a.Index)
}

func (h handlers) GetIssueAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetIssueAttachmentFn")
	var a issueAttachmentArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	return h.get(ctx, a.Owner, a.Repo, appattachment.ScopeIssue, a.Index, a.AttachmentID)
}

func (h handlers) DownloadIssueAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DownloadIssueAttachmentFn")
	var a issueAttachmentArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	return h.download(ctx, a.Owner, a.Repo, appattachment.ScopeIssue, a.Index, a.AttachmentID)
}

func (h handlers) CreateIssueAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateIssueAttachmentFn")
	var a issueCreateArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	return h.create(ctx, req, a.Owner, a.Repo, appattachment.ScopeIssue, a.Index, a.MimeType)
}

func (h handlers) EditIssueAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called EditIssueAttachmentFn")
	var a issueEditArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	return h.edit(ctx, a.Owner, a.Repo, appattachment.ScopeIssue, a.Index, a.AttachmentID, a.Name)
}

func (h handlers) DeleteIssueAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteIssueAttachmentFn")
	var a issueAttachmentArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	return h.remove(ctx, a.Owner, a.Repo, appattachment.ScopeIssue, a.Index, a.AttachmentID)
}

// --- Comment-scoped handlers ------------------------------------------------

func (h handlers) ListCommentAttachmentsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListCommentAttachmentsFn")
	var a commentRefArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	return h.list(ctx, a.Owner, a.Repo, appattachment.ScopeComment, a.CommentID)
}

func (h handlers) GetCommentAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetCommentAttachmentFn")
	var a commentAttachmentArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	return h.get(ctx, a.Owner, a.Repo, appattachment.ScopeComment, a.CommentID, a.AttachmentID)
}

func (h handlers) DownloadCommentAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DownloadCommentAttachmentFn")
	var a commentAttachmentArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	return h.download(ctx, a.Owner, a.Repo, appattachment.ScopeComment, a.CommentID, a.AttachmentID)
}

func (h handlers) CreateCommentAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateCommentAttachmentFn")
	var a commentCreateArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	return h.create(ctx, req, a.Owner, a.Repo, appattachment.ScopeComment, a.CommentID, a.MimeType)
}

func (h handlers) EditCommentAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called EditCommentAttachmentFn")
	var a commentEditArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	return h.edit(ctx, a.Owner, a.Repo, appattachment.ScopeComment, a.CommentID, a.AttachmentID, a.Name)
}

func (h handlers) DeleteCommentAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteCommentAttachmentFn")
	var a commentAttachmentArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	return h.remove(ctx, a.Owner, a.Repo, appattachment.ScopeComment, a.CommentID, a.AttachmentID)
}

// --- shared flows -----------------------------------------------------------
//
// Each flow takes the parent (index for an issue, comment_id for a comment)
// and attachment_id where a tool takes one, as the raw argument values; they
// are read in the order the handlers always read them, so the first failure
// is the one the client sees.

func (h handlers) list(ctx context.Context, owner, repo string, sc appattachment.Scope, parent any) (*mcp.CallToolResult, error) {
	p, err := number(parent, parentName(sc))
	if err != nil {
		return to.ErrorResult(err)
	}
	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	out, err := svc.List(ctx, owner, repo, sc, p)
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(out)
}

func (h handlers) get(ctx context.Context, owner, repo string, sc appattachment.Scope, parent, attachmentID any) (*mcp.CallToolResult, error) {
	p, err := number(parent, parentName(sc))
	if err != nil {
		return to.ErrorResult(err)
	}
	id, err := number(attachmentID, "attachment_id")
	if err != nil {
		return to.ErrorResult(err)
	}
	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	att, err := svc.Get(ctx, owner, repo, sc, p, id)
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(att)
}

func (h handlers) download(ctx context.Context, owner, repo string, sc appattachment.Scope, parent, attachmentID any) (*mcp.CallToolResult, error) {
	p, err := number(parent, parentName(sc))
	if err != nil {
		return to.ErrorResult(err)
	}
	id, err := number(attachmentID, "attachment_id")
	if err != nil {
		return to.ErrorResult(err)
	}
	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	dl, err := svc.Download(ctx, owner, repo, sc, p, id)
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

// create takes req as well as the decoded fields: the upload source is read
// from the raw arguments (see issueCreateArgs), after the parent is checked.
func (h handlers) create(ctx context.Context, req mcp.CallToolRequest, owner, repo string, sc appattachment.Scope, parent any, mimeType string) (*mcp.CallToolResult, error) {
	p, err := number(parent, parentName(sc))
	if err != nil {
		return to.ErrorResult(err)
	}
	reader, filename, err := upload.Open(upload.SourceFromArguments(req.GetArguments()))
	if err != nil {
		return to.ErrorResult(err)
	}
	defer reader.Close()

	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	att, err := svc.Create(ctx, owner, repo, sc, p, appattachment.Upload{Filename: filename, MIMEType: mimeType, Content: reader})
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(att)
}

func (h handlers) edit(ctx context.Context, owner, repo string, sc appattachment.Scope, parent, attachmentID any, name string) (*mcp.CallToolResult, error) {
	p, err := number(parent, parentName(sc))
	if err != nil {
		return to.ErrorResult(err)
	}
	id, err := number(attachmentID, "attachment_id")
	if err != nil {
		return to.ErrorResult(err)
	}
	if name == "" {
		return to.ErrorResult(fmt.Errorf("name is required"))
	}
	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	att, err := svc.Edit(ctx, owner, repo, sc, p, id, name)
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(att)
}

func (h handlers) remove(ctx context.Context, owner, repo string, sc appattachment.Scope, parent, attachmentID any) (*mcp.CallToolResult, error) {
	p, err := number(parent, parentName(sc))
	if err != nil {
		return to.ErrorResult(err)
	}
	id, err := number(attachmentID, "attachment_id")
	if err != nil {
		return to.ErrorResult(err)
	}
	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	if err := svc.Delete(ctx, owner, repo, sc, p, id); err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(map[string]string{"status": "deleted"})
}

// parentName is the argument that names the parent of a scope's attachments.
func parentName(sc appattachment.Scope) string {
	if sc == appattachment.ScopeComment {
		return "comment_id"
	}
	return "index"
}
