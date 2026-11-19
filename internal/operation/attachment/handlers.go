// SPDX-License-Identifier: GPL-3.0-or-later

package attachment

import (
	"context"
	"encoding/base64"
	"errors"
	"time"

	appattachment "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/attachment"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/model"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge/attachmentadapter"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/numarg"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/decode"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/mcpresult"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/pipeline"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/to"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/upload"

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

// RegisterTool registers all 12 attachment tools with the MCP server. Each
// request takes its Forgejo port from f through the request pipeline's Binder,
// so it runs under the credential that request carries, and only after the
// request is valid. The upload tools ask for the raw request (WithRaw), because
// their source is read from it.
func RegisterTool(s *server.MCPServer, f forge.Factory) {
	h := handlers{forge: f}
	s.AddTool(ListIssueAttachmentsTool, pipeline.Handle(h.service, listIssueAttachments))
	s.AddTool(GetIssueAttachmentTool, pipeline.Handle(h.service, getIssueAttachment))
	s.AddTool(DownloadIssueAttachmentTool, pipeline.Handle(h.service, downloadIssueAttachment))
	s.AddTool(CreateIssueAttachmentTool, pipeline.Handle(h.uploadService, createIssueAttachment, pipeline.WithRaw()))
	s.AddTool(EditIssueAttachmentTool, pipeline.Handle(h.service, editIssueAttachment))
	s.AddTool(DeleteIssueAttachmentTool, pipeline.Handle(h.service, deleteIssueAttachment))

	s.AddTool(ListCommentAttachmentsTool, pipeline.Handle(h.service, listCommentAttachments))
	s.AddTool(GetCommentAttachmentTool, pipeline.Handle(h.service, getCommentAttachment))
	s.AddTool(DownloadCommentAttachmentTool, pipeline.Handle(h.service, downloadCommentAttachment))
	s.AddTool(CreateCommentAttachmentTool, pipeline.Handle(h.uploadService, createCommentAttachment, pipeline.WithRaw()))
	s.AddTool(EditCommentAttachmentTool, pipeline.Handle(h.service, editCommentAttachment))
	s.AddTool(DeleteCommentAttachmentTool, pipeline.Handle(h.service, deleteCommentAttachment))
}

// handlers hold the factory rather than a port so each request gets the port
// for its own credential.
type handlers struct{ forge forge.Factory }

// service is the Binder for every attachment tool: it looks up the request's
// credential and binds the attachment use cases to it.
// uploadService is the Binder of the upload tools. It checks the upload source
// in the raw request before the credential lookup, so a bad source is refused
// as the source error, never as a credential error.
func (h handlers) uploadService(ctx context.Context) (*appattachment.Service, error) {
	raw, ok := pipeline.Raw(ctx)
	if !ok {
		return nil, errors.New("upload: the raw request is not available")
	}
	if err := upload.CheckArguments(raw.GetArguments()); err != nil {
		return nil, err
	}
	return h.service(ctx)
}

func (h handlers) service(ctx context.Context) (*appattachment.Service, error) {
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
// which the lax decoder would otherwise hide as zero. Each struct's Validate method
// holds the checks the tool made before its credential lookup; the pipeline
// runs them first.

type issueRefArgs struct {
	Owner string  `json:"owner" required:"true"`
	Repo  string  `json:"repo" required:"true"`
	Index any     `json:"index" required:"true" schema:"number"`
	Page  float64 `json:"page"`
	Limit float64 `json:"limit"`
}

type commentRefArgs struct {
	Owner     string  `json:"owner" required:"true"`
	Repo      string  `json:"repo" required:"true"`
	CommentID any     `json:"comment_id" required:"true" schema:"number"`
	Page      float64 `json:"page"`
	Limit     float64 `json:"limit"`
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
// arguments by upload.SourceFromArguments: it must tell a present non-string
// value (rejected, including null) from an absent one, which the lax decoder cannot.
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

// Validate checks the request before the credential lookup, in the order the
// tools always have: the issue index, then the paging.

func (a issueRefArgs) Validate() error {
	if err := decode.RequiredRepo(a.Owner, a.Repo); err != nil {
		return err
	}
	if _, err := number(a.Index, "index"); err != nil {
		return err
	}
	_, _, err := decode.PagingArgs(a.Page, a.Limit)
	return err
}

func (a commentRefArgs) Validate() error {
	if err := decode.RequiredRepo(a.Owner, a.Repo); err != nil {
		return err
	}
	if _, err := number(a.CommentID, "comment_id"); err != nil {
		return err
	}
	_, _, err := decode.PagingArgs(a.Page, a.Limit)
	return err
}

func (a issueAttachmentArgs) Validate() error {
	if err := decode.RequiredRepo(a.Owner, a.Repo); err != nil {
		return err
	}
	return parentAndAttachment(a.Index, "index", a.AttachmentID)
}

func (a commentAttachmentArgs) Validate() error {
	if err := decode.RequiredRepo(a.Owner, a.Repo); err != nil {
		return err
	}
	return parentAndAttachment(a.CommentID, "comment_id", a.AttachmentID)
}

func (a issueCreateArgs) Validate() error {
	if err := decode.RequiredRepo(a.Owner, a.Repo); err != nil {
		return err
	}
	_, err := number(a.Index, "index")
	return err
}

func (a commentCreateArgs) Validate() error {
	if err := decode.RequiredRepo(a.Owner, a.Repo); err != nil {
		return err
	}
	_, err := number(a.CommentID, "comment_id")
	return err
}

func (a issueEditArgs) Validate() error {
	if err := decode.RequiredRepo(a.Owner, a.Repo); err != nil {
		return err
	}
	return editOK(a.Index, "index", a.AttachmentID, a.Name)
}

func (a commentEditArgs) Validate() error {
	if err := decode.RequiredRepo(a.Owner, a.Repo); err != nil {
		return err
	}
	return editOK(a.CommentID, "comment_id", a.AttachmentID, a.Name)
}

// number reads a required ID through the numeric-argument module: a missing one
// is "<name> is required", a zero, negative or fractional one is "<name> must
// be a positive integer".
func number(v any, name string) (int64, error) {
	return numarg.RequiredPositiveInt(name, v)
}

func parentAndAttachment(parent any, parentName string, attachmentID any) error {
	if _, err := number(parent, parentName); err != nil {
		return err
	}
	_, err := number(attachmentID, "attachment_id")
	return err
}

func editOK(parent any, parentName string, attachmentID any, name string) error {
	if err := parentAndAttachment(parent, parentName, attachmentID); err != nil {
		return err
	}
	if name == "" {
		return errors.New("name is required")
	}
	return nil
}

// --- Issue-scoped tools -----------------------------------------------------

func listIssueAttachments(ctx context.Context, svc *appattachment.Service, a issueRefArgs) (*mcp.CallToolResult, error) {
	return list(ctx, svc, a.Owner, a.Repo, appattachment.ScopeIssue, a.Index, a.Page, a.Limit)
}

func getIssueAttachment(ctx context.Context, svc *appattachment.Service, a issueAttachmentArgs) (*mcp.CallToolResult, error) {
	return get(ctx, svc, a.Owner, a.Repo, appattachment.ScopeIssue, a.Index, a.AttachmentID)
}

func downloadIssueAttachment(ctx context.Context, svc *appattachment.Service, a issueAttachmentArgs) (*mcp.CallToolResult, error) {
	return download(ctx, svc, a.Owner, a.Repo, appattachment.ScopeIssue, a.Index, a.AttachmentID)
}

func createIssueAttachment(ctx context.Context, svc *appattachment.Service, a issueCreateArgs) (*mcp.CallToolResult, error) {
	p, _ := number(a.Index, "index") // Validate refused a bad index before the credential lookup
	return create(ctx, svc, a.Owner, a.Repo, appattachment.ScopeIssue, p, a.MimeType)
}

func editIssueAttachment(ctx context.Context, svc *appattachment.Service, a issueEditArgs) (*mcp.CallToolResult, error) {
	return edit(ctx, svc, a.Owner, a.Repo, appattachment.ScopeIssue, a.Index, a.AttachmentID, a.Name)
}

func deleteIssueAttachment(ctx context.Context, svc *appattachment.Service, a issueAttachmentArgs) (*mcp.CallToolResult, error) {
	return remove(ctx, svc, a.Owner, a.Repo, appattachment.ScopeIssue, a.Index, a.AttachmentID)
}

// --- Comment-scoped tools ---------------------------------------------------

func listCommentAttachments(ctx context.Context, svc *appattachment.Service, a commentRefArgs) (*mcp.CallToolResult, error) {
	return list(ctx, svc, a.Owner, a.Repo, appattachment.ScopeComment, a.CommentID, a.Page, a.Limit)
}

func getCommentAttachment(ctx context.Context, svc *appattachment.Service, a commentAttachmentArgs) (*mcp.CallToolResult, error) {
	return get(ctx, svc, a.Owner, a.Repo, appattachment.ScopeComment, a.CommentID, a.AttachmentID)
}

func downloadCommentAttachment(ctx context.Context, svc *appattachment.Service, a commentAttachmentArgs) (*mcp.CallToolResult, error) {
	return download(ctx, svc, a.Owner, a.Repo, appattachment.ScopeComment, a.CommentID, a.AttachmentID)
}

func createCommentAttachment(ctx context.Context, svc *appattachment.Service, a commentCreateArgs) (*mcp.CallToolResult, error) {
	p, _ := number(a.CommentID, "comment_id") // Validate refused a bad comment_id before the credential lookup
	return create(ctx, svc, a.Owner, a.Repo, appattachment.ScopeComment, p, a.MimeType)
}

func editCommentAttachment(ctx context.Context, svc *appattachment.Service, a commentEditArgs) (*mcp.CallToolResult, error) {
	return edit(ctx, svc, a.Owner, a.Repo, appattachment.ScopeComment, a.CommentID, a.AttachmentID, a.Name)
}

func deleteCommentAttachment(ctx context.Context, svc *appattachment.Service, a commentAttachmentArgs) (*mcp.CallToolResult, error) {
	return remove(ctx, svc, a.Owner, a.Repo, appattachment.ScopeComment, a.CommentID, a.AttachmentID)
}

// --- shared flows -----------------------------------------------------------
//
// Each flow takes the parent (index for an issue, comment_id for a comment)
// and attachment_id where a tool takes one, as the raw argument values. Validate
// has already checked them; the flows convert them to the use case's numbers.

func list(ctx context.Context, svc *appattachment.Service, owner, repo string, sc appattachment.Scope, parent any, page, limit float64) (*mcp.CallToolResult, error) {
	p, err := number(parent, parentName(sc))
	if err != nil {
		return mcpresult.Fail(err)
	}
	pg, lim, err := decode.PagingArgs(page, limit)
	if err != nil {
		return mcpresult.Fail(err)
	}
	out, err := svc.List(ctx, owner, repo, sc, p, appattachment.Paging{Page: pg, Limit: lim})
	if err != nil {
		return mcpresult.Fail(err)
	}
	return to.TextResult(model.AttachmentPageOf(out))
}

func get(ctx context.Context, svc *appattachment.Service, owner, repo string, sc appattachment.Scope, parent, attachmentID any) (*mcp.CallToolResult, error) {
	p, id, err := parentAndID(parent, sc, attachmentID)
	if err != nil {
		return mcpresult.Fail(err)
	}
	att, err := svc.Get(ctx, owner, repo, sc, p, id)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return to.TextResult(att)
}

func download(ctx context.Context, svc *appattachment.Service, owner, repo string, sc appattachment.Scope, parent, attachmentID any) (*mcp.CallToolResult, error) {
	p, id, err := parentAndID(parent, sc, attachmentID)
	if err != nil {
		return mcpresult.Fail(err)
	}
	dl, err := svc.Download(ctx, owner, repo, sc, p, id)
	if err != nil {
		return mcpresult.Fail(err)
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

// create reads the upload source from the raw request (see issueCreateArgs),
// which uploadService has already checked, and opens the file here, after the
// credential lookup.
func create(ctx context.Context, svc *appattachment.Service, owner, repo string, sc appattachment.Scope, p int64, mimeType string) (*mcp.CallToolResult, error) {
	raw, ok := pipeline.Raw(ctx)
	if !ok {
		return mcpresult.Fail(errors.New("upload: the raw request is not available"))
	}
	source, err := upload.SourceFromArguments(raw.GetArguments())
	if err != nil {
		return mcpresult.Fail(err)
	}
	reader, filename, err := upload.Open(source)
	if err != nil {
		return mcpresult.Fail(err)
	}
	defer reader.Close()

	att, err := svc.Create(ctx, owner, repo, sc, p, appattachment.Upload{Filename: filename, MIMEType: mimeType, Content: reader})
	if err != nil {
		return mcpresult.Fail(err)
	}
	return to.TextResult(att)
}

func edit(ctx context.Context, svc *appattachment.Service, owner, repo string, sc appattachment.Scope, parent, attachmentID any, name string) (*mcp.CallToolResult, error) {
	p, id, err := parentAndID(parent, sc, attachmentID)
	if err != nil {
		return mcpresult.Fail(err)
	}
	att, err := svc.Edit(ctx, owner, repo, sc, p, id, name)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return to.TextResult(att)
}

func remove(ctx context.Context, svc *appattachment.Service, owner, repo string, sc appattachment.Scope, parent, attachmentID any) (*mcp.CallToolResult, error) {
	p, id, err := parentAndID(parent, sc, attachmentID)
	if err != nil {
		return mcpresult.Fail(err)
	}
	if err := svc.Delete(ctx, owner, repo, sc, p, id); err != nil {
		return mcpresult.Fail(err)
	}
	return to.TextResult(map[string]string{"status": "deleted"})
}

// parentAndID converts the parent and attachment numbers of a scope's tool.
func parentAndID(parent any, sc appattachment.Scope, attachmentID any) (int64, int64, error) {
	p, err := number(parent, parentName(sc))
	if err != nil {
		return 0, 0, err
	}
	id, err := number(attachmentID, "attachment_id")
	if err != nil {
		return 0, 0, err
	}
	return p, id, nil
}

// parentName is the argument that names the parent of a scope's attachments.
func parentName(sc appattachment.Scope) string {
	if sc == appattachment.ScopeComment {
		return "comment_id"
	}
	return "index"
}
