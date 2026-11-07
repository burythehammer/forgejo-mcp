// SPDX-License-Identifier: GPL-3.0-or-later

package release

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	apprelease "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/release"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/releaseadapter"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/numarg"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/decode"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcpresult"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/pipeline"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/to"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/upload"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterTool registers all the release tools with the MCP server. Each
// request takes its Forgejo port from f through the request pipeline's Binder,
// so it runs under the credential that request carries, and only after the
// request is valid. The upload tool asks for the raw request (WithRaw), because
// its source is read from it.
func RegisterTool(s *server.MCPServer, f forge.Factory) {
	h := handlers{forge: f}
	s.AddTool(ListReleasesTool, pipeline.Handle(h.service, listReleases))
	s.AddTool(GetReleaseByIDTool, pipeline.Handle(h.service, getReleaseByID))
	s.AddTool(GetReleaseByTagTool, pipeline.Handle(h.service, getReleaseByTag))
	s.AddTool(GetLatestReleaseTool, pipeline.Handle(h.service, getLatestRelease))
	s.AddTool(CreateReleaseTool, pipeline.Handle(h.service, createRelease))
	s.AddTool(EditReleaseTool, pipeline.Handle(h.service, editRelease))
	s.AddTool(DeleteReleaseTool, pipeline.Handle(h.service, deleteRelease))
	s.AddTool(DeleteReleaseByTagTool, pipeline.Handle(h.service, deleteReleaseByTag))

	s.AddTool(ListReleaseAttachmentsTool, pipeline.Handle(h.service, listReleaseAttachments))
	s.AddTool(GetReleaseAttachmentTool, pipeline.Handle(h.service, getReleaseAttachment))
	s.AddTool(DownloadReleaseAttachmentTool, pipeline.Handle(h.service, downloadReleaseAttachment))
	s.AddTool(CreateReleaseAttachmentTool, pipeline.Handle(h.uploadService, createReleaseAttachment, pipeline.WithRaw()))
	s.AddTool(EditReleaseAttachmentTool, pipeline.Handle(h.service, editReleaseAttachment))
	s.AddTool(DeleteReleaseAttachmentTool, pipeline.Handle(h.service, deleteReleaseAttachment))
}

// handlers hold the factory rather than a port so each request gets the port
// for its own credential.
type handlers struct{ forge forge.Factory }

// service is the Binder for every release tool: it looks up the request's
// credential and binds the release use cases to it.
// uploadService is the Binder of the upload tool. It checks the upload source
// in the raw request before the credential lookup, so a bad source is refused
// as the source error, never as a credential error.
func (h handlers) uploadService(ctx context.Context) (*apprelease.Service, error) {
	raw, ok := pipeline.Raw(ctx)
	if !ok {
		return nil, errors.New("upload: the raw request is not available")
	}
	if err := upload.CheckArguments(raw.GetArguments()); err != nil {
		return nil, err
	}
	return h.service(ctx)
}

func (h handlers) service(ctx context.Context) (*apprelease.Service, error) {
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
// by number(). the lax decoder would otherwise hide that failure as zero. Optional
// numbers are float64: the lax decoder reads them as the old to.Float64 calls did.
// Each struct's Validate method holds the checks the tool made before its
// credential lookup; the pipeline runs them first.

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
// including null) from an absent one, which the lax decoder cannot.
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

// Validate checks the request before the credential lookup, as each tool
// always has.

func (a listReleasesArgs) Validate() error {
	if err := decode.RequiredRepo(a.Owner, a.Repo); err != nil {
		return err
	}
	if _, err := stateOf(a.State); err != nil {
		return err
	}
	_, _, err := decode.PagingArgs(a.Page, a.Limit)
	return err
}

func (a repoArgs) Validate() error { return decode.RequiredRepo(a.Owner, a.Repo) }

func (a releaseRefArgs) Validate() error {
	if err := decode.RequiredRepo(a.Owner, a.Repo); err != nil {
		return err
	}
	_, err := number(a.ReleaseID, "release_id")
	return err
}

func (a tagRefArgs) Validate() error {
	if err := decode.RequiredRepo(a.Owner, a.Repo); err != nil {
		return err
	}
	if a.Tag == "" {
		return errors.New("tag is required")
	}
	return nil
}

func (a createReleaseArgs) Validate() error {
	if err := decode.RequiredRepo(a.Owner, a.Repo); err != nil {
		return err
	}
	if a.TagName == "" {
		return errors.New("tag_name is required")
	}
	return nil
}

func (a editReleaseArgs) Validate() error {
	if err := decode.RequiredRepo(a.Owner, a.Repo); err != nil {
		return err
	}
	_, err := number(a.ReleaseID, "release_id")
	return err
}

func (a listAttachmentsArgs) Validate() error {
	if err := decode.RequiredRepo(a.Owner, a.Repo); err != nil {
		return err
	}
	if _, err := number(a.ReleaseID, "release_id"); err != nil {
		return err
	}
	_, _, err := decode.PagingArgs(a.Page, a.Limit)
	return err
}

func (a attachmentRefArgs) Validate() error {
	if err := decode.RequiredRepo(a.Owner, a.Repo); err != nil {
		return err
	}
	_, _, err := releaseAndAttachment(a.ReleaseID, a.AttachmentID)
	return err
}

func (a createAttachmentArgs) Validate() error {
	if err := decode.RequiredRepo(a.Owner, a.Repo); err != nil {
		return err
	}
	_, err := number(a.ReleaseID, "release_id")
	return err
}

func (a editAttachmentArgs) Validate() error {
	if err := decode.RequiredRepo(a.Owner, a.Repo); err != nil {
		return err
	}
	if _, _, err := releaseAndAttachment(a.ReleaseID, a.AttachmentID); err != nil {
		return err
	}
	if a.Name == "" {
		return errors.New("name is required")
	}
	return nil
}

// stateOf checks the list filter: empty means all, anything else must be one
// of the four states.
func stateOf(state string) (string, error) {
	if state == "" {
		state = stateAll
	}
	switch state {
	case stateAll, stateDraft, statePrerelease, statePublished:
		return state, nil
	default:
		return "", fmt.Errorf("invalid state %q: must be one of all|draft|prerelease|published", state)
	}
}

// number reads a required numeric argument, with the tool's error text.
func number(v any, name string) (int64, error) {
	f, err := to.Float64(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	return numarg.PositiveID(name, f)
}

// releaseAndAttachment converts the release and attachment numbers of an
// attachment tool, in the order the tools always have.
func releaseAndAttachment(releaseID, attachmentID any) (int64, int64, error) {
	rid, err := number(releaseID, "release_id")
	if err != nil {
		return 0, 0, err
	}
	aid, err := number(attachmentID, "attachment_id")
	if err != nil {
		return 0, 0, err
	}
	return rid, aid, nil
}

// --- Release tools ----------------------------------------------------------

func listReleases(ctx context.Context, svc *apprelease.Service, a listReleasesArgs) (*mcp.CallToolResult, error) {
	state, err := stateOf(a.State)
	if err != nil {
		return mcpresult.Fail(err)
	}
	pg, lim, err := decode.PagingArgs(a.Page, a.Limit)
	if err != nil {
		return mcpresult.Fail(err)
	}
	page, err := svc.ListReleases(ctx, a.Owner, a.Repo, apprelease.ListQuery{
		State:  state,
		Paging: apprelease.Paging{Page: pg, Limit: lim},
	})
	if err != nil {
		return mcpresult.Fail(err)
	}
	return to.TextResult(model.ReleasePage{Releases: page.Items, PageMeta: model.PageMetaOf(page)})
}

func getReleaseByID(ctx context.Context, svc *apprelease.Service, a releaseRefArgs) (*mcp.CallToolResult, error) {
	rid, err := number(a.ReleaseID, "release_id")
	if err != nil {
		return mcpresult.Fail(err)
	}
	rel, err := svc.GetRelease(ctx, a.Owner, a.Repo, rid)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return to.TextResult(rel)
}

func getReleaseByTag(ctx context.Context, svc *apprelease.Service, a tagRefArgs) (*mcp.CallToolResult, error) {
	rel, err := svc.GetReleaseByTag(ctx, a.Owner, a.Repo, a.Tag)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return to.TextResult(rel)
}

func getLatestRelease(ctx context.Context, svc *apprelease.Service, a repoArgs) (*mcp.CallToolResult, error) {
	rel, err := svc.GetLatestRelease(ctx, a.Owner, a.Repo)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return to.TextResult(rel)
}

func createRelease(ctx context.Context, svc *apprelease.Service, a createReleaseArgs) (*mcp.CallToolResult, error) {
	rel, err := svc.CreateRelease(ctx, a.Owner, a.Repo, apprelease.CreateInput{
		TagName: a.TagName, Target: a.TargetCommitish, Title: a.Name, Note: a.Body,
		IsDraft: a.Draft, IsPrerelease: a.Prerelease,
	})
	if err != nil {
		return mcpresult.Fail(err)
	}
	return to.TextResult(rel)
}

func editRelease(ctx context.Context, svc *apprelease.Service, a editReleaseArgs) (*mcp.CallToolResult, error) {
	rid, err := number(a.ReleaseID, "release_id")
	if err != nil {
		return mcpresult.Fail(err)
	}
	in := apprelease.EditInput{
		TagName:      a.TagName,
		Target:       a.TargetCommitish,
		Title:        a.Name,
		Note:         a.Body,
		IsDraft:      a.Draft,
		IsPrerelease: a.Prerelease,
	}
	rel, err := svc.EditRelease(ctx, a.Owner, a.Repo, rid, in)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return to.TextResult(rel)
}

func deleteRelease(ctx context.Context, svc *apprelease.Service, a releaseRefArgs) (*mcp.CallToolResult, error) {
	rid, err := number(a.ReleaseID, "release_id")
	if err != nil {
		return mcpresult.Fail(err)
	}
	if err := svc.DeleteRelease(ctx, a.Owner, a.Repo, rid); err != nil {
		return mcpresult.Fail(err)
	}
	return to.TextResult(map[string]string{"status": "deleted"})
}

func deleteReleaseByTag(ctx context.Context, svc *apprelease.Service, a tagRefArgs) (*mcp.CallToolResult, error) {
	if err := svc.DeleteReleaseByTag(ctx, a.Owner, a.Repo, a.Tag); err != nil {
		return mcpresult.Fail(err)
	}
	return to.TextResult(map[string]string{"status": "deleted"})
}

// --- Release attachment tools ----------------------------------------------

func listReleaseAttachments(ctx context.Context, svc *apprelease.Service, a listAttachmentsArgs) (*mcp.CallToolResult, error) {
	rid, err := number(a.ReleaseID, "release_id")
	if err != nil {
		return mcpresult.Fail(err)
	}
	pg, lim, err := decode.PagingArgs(a.Page, a.Limit)
	if err != nil {
		return mcpresult.Fail(err)
	}
	page, err := svc.ListAttachments(ctx, a.Owner, a.Repo, rid, apprelease.Paging{Page: pg, Limit: lim})
	if err != nil {
		return mcpresult.Fail(err)
	}
	return to.TextResult(model.AttachmentPageOf(page))
}

func getReleaseAttachment(ctx context.Context, svc *apprelease.Service, a attachmentRefArgs) (*mcp.CallToolResult, error) {
	rid, aid, err := releaseAndAttachment(a.ReleaseID, a.AttachmentID)
	if err != nil {
		return mcpresult.Fail(err)
	}
	att, err := svc.GetAttachment(ctx, a.Owner, a.Repo, rid, aid)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return to.TextResult(att)
}

func downloadReleaseAttachment(ctx context.Context, svc *apprelease.Service, a attachmentRefArgs) (*mcp.CallToolResult, error) {
	rid, aid, err := releaseAndAttachment(a.ReleaseID, a.AttachmentID)
	if err != nil {
		return mcpresult.Fail(err)
	}
	dl, err := svc.DownloadAttachment(ctx, a.Owner, a.Repo, rid, aid)
	if err != nil {
		return mcpresult.Fail(err)
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

// createReleaseAttachment reads the upload source from the raw request (see
// createAttachmentArgs), which uploadService has already checked, and opens the
// file here, after the credential lookup.
func createReleaseAttachment(ctx context.Context, svc *apprelease.Service, a createAttachmentArgs) (*mcp.CallToolResult, error) {
	rid, _ := number(a.ReleaseID, "release_id") // Validate refused a bad release_id before the credential lookup
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

	att, err := svc.CreateAttachment(ctx, a.Owner, a.Repo, rid, apprelease.Upload{Filename: filename, MIMEType: a.MimeType, Content: reader})
	if err != nil {
		return mcpresult.Fail(err)
	}
	return to.TextResult(att)
}

func editReleaseAttachment(ctx context.Context, svc *apprelease.Service, a editAttachmentArgs) (*mcp.CallToolResult, error) {
	rid, aid, err := releaseAndAttachment(a.ReleaseID, a.AttachmentID)
	if err != nil {
		return mcpresult.Fail(err)
	}
	att, err := svc.EditAttachment(ctx, a.Owner, a.Repo, rid, aid, a.Name)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return to.TextResult(att)
}

func deleteReleaseAttachment(ctx context.Context, svc *apprelease.Service, a attachmentRefArgs) (*mcp.CallToolResult, error) {
	rid, aid, err := releaseAndAttachment(a.ReleaseID, a.AttachmentID)
	if err != nil {
		return mcpresult.Fail(err)
	}
	if err := svc.DeleteAttachment(ctx, a.Owner, a.Repo, rid, aid); err != nil {
		return mcpresult.Fail(err)
	}
	return to.TextResult(map[string]string{"status": "deleted"})
}
