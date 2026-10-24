// SPDX-License-Identifier: GPL-3.0-or-later

package release

import (
	"context"
	"encoding/base64"
	"fmt"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	apprelease "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/release"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/releaseadapter"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/log"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/numarg"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/decode"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/to"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/upload"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterTool registers all the release tools with the MCP server.
func RegisterTool(s *server.MCPServer, f forge.Factory) {
	h := handlers{forge: f}
	s.AddTool(ListReleasesTool, h.ListReleasesFn)
	s.AddTool(GetReleaseByIDTool, h.GetReleaseByIDFn)
	s.AddTool(GetReleaseByTagTool, h.GetReleaseByTagFn)
	s.AddTool(GetLatestReleaseTool, h.GetLatestReleaseFn)
	s.AddTool(CreateReleaseTool, h.CreateReleaseFn)
	s.AddTool(EditReleaseTool, h.EditReleaseFn)
	s.AddTool(DeleteReleaseTool, h.DeleteReleaseFn)
	s.AddTool(DeleteReleaseByTagTool, h.DeleteReleaseByTagFn)

	s.AddTool(ListReleaseAttachmentsTool, h.ListReleaseAttachmentsFn)
	s.AddTool(GetReleaseAttachmentTool, h.GetReleaseAttachmentFn)
	s.AddTool(DownloadReleaseAttachmentTool, h.DownloadReleaseAttachmentFn)
	s.AddTool(CreateReleaseAttachmentTool, h.CreateReleaseAttachmentFn)
	s.AddTool(EditReleaseAttachmentTool, h.EditReleaseAttachmentFn)
	s.AddTool(DeleteReleaseAttachmentTool, h.DeleteReleaseAttachmentFn)
}

// handlers serve the release tools. They hold the factory rather than a port so
// each request gets the port for its own credential. Each handler is the MCP
// adapter of one use case in internal/app/release: read the arguments, run the use case,
// render the result.
type handlers struct{ forge forge.Factory }

// useCases returns the release use cases over the request's Forgejo port.
func useCases(ctx context.Context, h handlers) (*apprelease.Service, error) {
	port, err := h.forge.For(ctx)
	if err != nil {
		return nil, err
	}
	return apprelease.New(releaseadapter.New(port, port)), nil
}

// Argument structs, one per tool shape. `json` names the MCP argument and
// `required:"true"` marks the required ones; mcptest.ArgsMatchSchema keeps them
// in agreement with the tool definitions.
//
// A number that is required and must fail with its own "<name>: <cause>" text
// when missing or wrongly typed is kept as `any` (with schema:"number") and read
// by number(). decode.Lax would otherwise hide that failure as zero. Optional
// numbers are float64: decode.Lax reads them as the old to.Float64 calls did.

type repoArgs struct {
	Owner string `json:"owner" required:"true"`
	Repo  string `json:"repo" required:"true"`
}

type releaseRefArgs struct {
	Owner     string `json:"owner" required:"true"`
	Repo      string `json:"repo" required:"true"`
	ReleaseID any    `json:"release_id" required:"true" schema:"number"`
}

type tagRefArgs struct {
	Owner string `json:"owner" required:"true"`
	Repo  string `json:"repo" required:"true"`
	Tag   string `json:"tag" required:"true"`
}

type listReleasesArgs struct {
	Owner string  `json:"owner" required:"true"`
	Repo  string  `json:"repo" required:"true"`
	Page  float64 `json:"page"`
	Limit float64 `json:"limit"`
	State string  `json:"state"`
}

type createReleaseArgs struct {
	Owner           string `json:"owner" required:"true"`
	Repo            string `json:"repo" required:"true"`
	TagName         string `json:"tag_name" required:"true"`
	TargetCommitish string `json:"target_commitish"`
	Name            string `json:"name"`
	Body            string `json:"body"`
	Draft           bool   `json:"draft"`
	Prerelease      bool   `json:"prerelease"`
}

// editReleaseArgs keeps draft and prerelease as pointers: an absent or
// wrongly typed flag must leave the stored value alone.
type editReleaseArgs struct {
	Owner           string `json:"owner" required:"true"`
	Repo            string `json:"repo" required:"true"`
	ReleaseID       any    `json:"release_id" required:"true" schema:"number"`
	TagName         string `json:"tag_name"`
	TargetCommitish string `json:"target_commitish"`
	Name            string `json:"name"`
	Body            string `json:"body"`
	Draft           *bool  `json:"draft"`
	Prerelease      *bool  `json:"prerelease"`
}

type listAttachmentsArgs struct {
	Owner     string  `json:"owner" required:"true"`
	Repo      string  `json:"repo" required:"true"`
	ReleaseID any     `json:"release_id" required:"true" schema:"number"`
	Page      float64 `json:"page"`
	Limit     float64 `json:"limit"`
}

type attachmentRefArgs struct {
	Owner        string `json:"owner" required:"true"`
	Repo         string `json:"repo" required:"true"`
	ReleaseID    any    `json:"release_id" required:"true" schema:"number"`
	AttachmentID any    `json:"attachment_id" required:"true" schema:"number"`
}

// createAttachmentArgs declares content and file_path so the schema check covers
// them, but the upload source is read from the raw arguments by
// upload.SourceFromArguments: it must tell a present non-string value (rejected,
// including null) from an absent one, which decode.Lax cannot.
type createAttachmentArgs struct {
	Owner     string `json:"owner" required:"true"`
	Repo      string `json:"repo" required:"true"`
	ReleaseID any    `json:"release_id" required:"true" schema:"number"`
	Content   any    `json:"content" schema:"string"`
	FilePath  any    `json:"file_path" schema:"string"`
	Filename  string `json:"filename"`
	MimeType  string `json:"mime_type"`
}

type editAttachmentArgs struct {
	Owner        string `json:"owner" required:"true"`
	Repo         string `json:"repo" required:"true"`
	ReleaseID    any    `json:"release_id" required:"true" schema:"number"`
	AttachmentID any    `json:"attachment_id" required:"true" schema:"number"`
	Name         string `json:"name" required:"true"`
}

// number reads a required numeric argument, with the tool's error text.
func number(v any, name string) (int64, error) {
	f, err := to.Float64(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	return numarg.PositiveID(name, f)
}

// --- Release tools ----------------------------------------------------------

func (h handlers) ListReleasesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListReleasesFn")
	var a listReleasesArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	state := a.State
	if state == "" {
		state = stateAll
	}
	switch state {
	case stateAll, stateDraft, statePrerelease, statePublished:
	default:
		return to.ErrorResult(fmt.Errorf("invalid state %q: must be one of all|draft|prerelease|published", state))
	}

	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	pg, lim, perr := decode.PagingArgs(a.Page, a.Limit)
	if perr != nil {
		return to.ErrorResult(perr)
	}
	page, err := svc.ListReleases(ctx, a.Owner, a.Repo, apprelease.ListQuery{
		State:  state,
		Paging: apprelease.Paging{Page: pg, Limit: lim},
	})
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(model.ReleasePage{Releases: page.Items, PageMeta: model.PageMetaOf(page)})
}

func (h handlers) GetReleaseByIDFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetReleaseByIDFn")
	var a releaseRefArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	rid, err := number(a.ReleaseID, "release_id")
	if err != nil {
		return to.ErrorResult(err)
	}
	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	rel, err := svc.GetRelease(ctx, a.Owner, a.Repo, rid)
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(rel)
}

func (h handlers) GetReleaseByTagFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetReleaseByTagFn")
	var a tagRefArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	if a.Tag == "" {
		return to.ErrorResult(fmt.Errorf("tag is required"))
	}
	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	rel, err := svc.GetReleaseByTag(ctx, a.Owner, a.Repo, a.Tag)
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(rel)
}

func (h handlers) GetLatestReleaseFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetLatestReleaseFn")
	var a repoArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	rel, err := svc.GetLatestRelease(ctx, a.Owner, a.Repo)
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(rel)
}

func (h handlers) CreateReleaseFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateReleaseFn")
	var a createReleaseArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	if a.TagName == "" {
		return to.ErrorResult(fmt.Errorf("tag_name is required"))
	}

	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	rel, err := svc.CreateRelease(ctx, a.Owner, a.Repo, apprelease.CreateInput{
		TagName: a.TagName, Target: a.TargetCommitish, Title: a.Name, Note: a.Body,
		IsDraft: a.Draft, IsPrerelease: a.Prerelease,
	})
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(rel)
}

func (h handlers) EditReleaseFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called EditReleaseFn")
	var a editReleaseArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	rid, err := number(a.ReleaseID, "release_id")
	if err != nil {
		return to.ErrorResult(err)
	}

	in := apprelease.EditInput{
		TagName:      a.TagName,
		Target:       a.TargetCommitish,
		Title:        a.Name,
		Note:         a.Body,
		IsDraft:      a.Draft,
		IsPrerelease: a.Prerelease,
	}

	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	rel, err := svc.EditRelease(ctx, a.Owner, a.Repo, rid, in)
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(rel)
}

func (h handlers) DeleteReleaseFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteReleaseFn")
	var a releaseRefArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	rid, err := number(a.ReleaseID, "release_id")
	if err != nil {
		return to.ErrorResult(err)
	}
	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	if err := svc.DeleteRelease(ctx, a.Owner, a.Repo, rid); err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(map[string]string{"status": "deleted"})
}

func (h handlers) DeleteReleaseByTagFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteReleaseByTagFn")
	var a tagRefArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	if a.Tag == "" {
		return to.ErrorResult(fmt.Errorf("tag is required"))
	}
	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	if err := svc.DeleteReleaseByTag(ctx, a.Owner, a.Repo, a.Tag); err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(map[string]string{"status": "deleted"})
}

// --- Release attachment tools ----------------------------------------------

func (h handlers) ListReleaseAttachmentsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListReleaseAttachmentsFn")
	var a listAttachmentsArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	rid, err := number(a.ReleaseID, "release_id")
	if err != nil {
		return to.ErrorResult(err)
	}
	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	pg, lim, perr := decode.PagingArgs(a.Page, a.Limit)
	if perr != nil {
		return to.ErrorResult(perr)
	}
	page, err := svc.ListAttachments(ctx, a.Owner, a.Repo, rid, apprelease.Paging{Page: pg, Limit: lim})
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(model.AttachmentPageOf(page))
}

func (h handlers) GetReleaseAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetReleaseAttachmentFn")
	var a attachmentRefArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	rid, err := number(a.ReleaseID, "release_id")
	if err != nil {
		return to.ErrorResult(err)
	}
	aid, err := number(a.AttachmentID, "attachment_id")
	if err != nil {
		return to.ErrorResult(err)
	}
	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	att, err := svc.GetAttachment(ctx, a.Owner, a.Repo, rid, aid)
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(att)
}

func (h handlers) DownloadReleaseAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DownloadReleaseAttachmentFn")
	var a attachmentRefArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	rid, err := number(a.ReleaseID, "release_id")
	if err != nil {
		return to.ErrorResult(err)
	}
	aid, err := number(a.AttachmentID, "attachment_id")
	if err != nil {
		return to.ErrorResult(err)
	}
	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	dl, err := svc.DownloadAttachment(ctx, a.Owner, a.Repo, rid, aid)
	if err != nil {
		return to.ErrorResult(err)
	}
	if !dl.Result.Inline {
		return to.TextResult(dl.Result)
	}
	// Inline bytes ride along as an embedded blob next to the JSON envelope.
	textPart := to.SafeJSONMarshal(dl.Result)
	return mcp.NewToolResultResource(textPart, mcp.BlobResourceContents{
		URI:      dl.Result.Attachment.DownloadURL,
		MIMEType: dl.Body.ContentType,
		Blob:     base64.StdEncoding.EncodeToString(dl.Body.Body),
	}), nil
}

func (h handlers) CreateReleaseAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateReleaseAttachmentFn")
	var a createAttachmentArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	rid, err := number(a.ReleaseID, "release_id")
	if err != nil {
		return to.ErrorResult(err)
	}
	source, err := upload.SourceFromArguments(req.GetArguments())
	if err != nil {
		return to.ErrorResult(err)
	}
	reader, filename, err := upload.Open(source)
	if err != nil {
		return to.ErrorResult(err)
	}
	defer reader.Close()

	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	att, err := svc.CreateAttachment(ctx, a.Owner, a.Repo, rid, apprelease.Upload{Filename: filename, MIMEType: a.MimeType, Content: reader})
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(att)
}

func (h handlers) EditReleaseAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called EditReleaseAttachmentFn")
	var a editAttachmentArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	rid, err := number(a.ReleaseID, "release_id")
	if err != nil {
		return to.ErrorResult(err)
	}
	aid, err := number(a.AttachmentID, "attachment_id")
	if err != nil {
		return to.ErrorResult(err)
	}
	if a.Name == "" {
		return to.ErrorResult(fmt.Errorf("name is required"))
	}
	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	att, err := svc.EditAttachment(ctx, a.Owner, a.Repo, rid, aid, a.Name)
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(att)
}

func (h handlers) DeleteReleaseAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteReleaseAttachmentFn")
	var a attachmentRefArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	rid, err := number(a.ReleaseID, "release_id")
	if err != nil {
		return to.ErrorResult(err)
	}
	aid, err := number(a.AttachmentID, "attachment_id")
	if err != nil {
		return to.ErrorResult(err)
	}
	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	if err := svc.DeleteAttachment(ctx, a.Owner, a.Repo, rid, aid); err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(map[string]string{"status": "deleted"})
}
