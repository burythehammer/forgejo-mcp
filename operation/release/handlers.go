// SPDX-License-Identifier: GPL-3.0-or-later

package release

import (
	"context"
	"encoding/base64"
	"fmt"

	apprelease "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/release"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/releaseadapter"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/to"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/upload"

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
// adapter of one use case in app/release: read the arguments, run the use case,
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

// --- Release tools ----------------------------------------------------------

func (h handlers) ListReleasesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListReleasesFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	page, _ := to.Float64(args["page"])
	if page == 0 {
		page = 1
	}
	limit, _ := to.Float64(args["limit"])
	if limit == 0 {
		limit = 20
	}
	state, ok := args["state"].(string)
	if !ok || state == "" {
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
	rels, err := svc.ListReleases(ctx, owner, repo, apprelease.ListQuery{
		State:  state,
		Paging: apprelease.Paging{Page: int(page), Limit: int(limit)},
	})
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(rels)
}

func (h handlers) GetReleaseByIDFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetReleaseByIDFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	rid, err := to.Float64(args["release_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("release_id: %w", err))
	}
	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	rel, err := svc.GetRelease(ctx, owner, repo, int64(rid))
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(rel)
}

func (h handlers) GetReleaseByTagFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetReleaseByTagFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	tag, _ := args["tag"].(string)
	if tag == "" {
		return to.ErrorResult(fmt.Errorf("tag is required"))
	}
	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	rel, err := svc.GetReleaseByTag(ctx, owner, repo, tag)
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(rel)
}

func (h handlers) GetLatestReleaseFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetLatestReleaseFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	rel, err := svc.GetLatestRelease(ctx, owner, repo)
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(rel)
}

func (h handlers) CreateReleaseFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateReleaseFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	tagName, _ := args["tag_name"].(string)
	if tagName == "" {
		return to.ErrorResult(fmt.Errorf("tag_name is required"))
	}
	target, _ := args["target_commitish"].(string)
	name, _ := args["name"].(string)
	body, _ := args["body"].(string)
	draft, _ := args["draft"].(bool)
	prerelease, _ := args["prerelease"].(bool)

	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	rel, err := svc.CreateRelease(ctx, owner, repo, apprelease.CreateInput{
		TagName: tagName, Target: target, Title: name, Note: body,
		IsDraft: draft, IsPrerelease: prerelease,
	})
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(rel)
}

func (h handlers) EditReleaseFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called EditReleaseFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	rid, err := to.Float64(args["release_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("release_id: %w", err))
	}

	var in apprelease.EditInput
	if v, ok := args["tag_name"].(string); ok {
		in.TagName = v
	}
	if v, ok := args["target_commitish"].(string); ok {
		in.Target = v
	}
	if v, ok := args["name"].(string); ok {
		in.Title = v
	}
	if v, ok := args["body"].(string); ok {
		in.Note = v
	}
	if v, ok := args["draft"].(bool); ok {
		in.IsDraft = &v
	}
	if v, ok := args["prerelease"].(bool); ok {
		in.IsPrerelease = &v
	}

	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	rel, err := svc.EditRelease(ctx, owner, repo, int64(rid), in)
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(rel)
}

func (h handlers) DeleteReleaseFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteReleaseFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	rid, err := to.Float64(args["release_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("release_id: %w", err))
	}
	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	if err := svc.DeleteRelease(ctx, owner, repo, int64(rid)); err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(map[string]string{"status": "deleted"})
}

func (h handlers) DeleteReleaseByTagFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteReleaseByTagFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	tag, _ := args["tag"].(string)
	if tag == "" {
		return to.ErrorResult(fmt.Errorf("tag is required"))
	}
	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	if err := svc.DeleteReleaseByTag(ctx, owner, repo, tag); err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(map[string]string{"status": "deleted"})
}

// --- Release attachment tools ----------------------------------------------

func (h handlers) ListReleaseAttachmentsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListReleaseAttachmentsFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	rid, err := to.Float64(args["release_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("release_id: %w", err))
	}
	page, _ := to.Float64(args["page"])
	if page == 0 {
		page = 1
	}
	limit, _ := to.Float64(args["limit"])
	if limit == 0 {
		limit = 20
	}

	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	list, err := svc.ListAttachments(ctx, owner, repo, int64(rid), apprelease.Paging{Page: int(page), Limit: int(limit)})
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(list)
}

func (h handlers) GetReleaseAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetReleaseAttachmentFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	rid, err := to.Float64(args["release_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("release_id: %w", err))
	}
	aid, err := to.Float64(args["attachment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("attachment_id: %w", err))
	}
	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	att, err := svc.GetAttachment(ctx, owner, repo, int64(rid), int64(aid))
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(att)
}

func (h handlers) DownloadReleaseAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DownloadReleaseAttachmentFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	rid, err := to.Float64(args["release_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("release_id: %w", err))
	}
	aid, err := to.Float64(args["attachment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("attachment_id: %w", err))
	}
	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	dl, err := svc.DownloadAttachment(ctx, owner, repo, int64(rid), int64(aid))
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
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	rid, err := to.Float64(args["release_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("release_id: %w", err))
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
	att, err := svc.CreateAttachment(ctx, owner, repo, int64(rid), apprelease.Upload{Filename: filename, MIMEType: mimeType, Content: reader})
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(att)
}

func (h handlers) EditReleaseAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called EditReleaseAttachmentFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	rid, err := to.Float64(args["release_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("release_id: %w", err))
	}
	aid, err := to.Float64(args["attachment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("attachment_id: %w", err))
	}
	name, _ := args["name"].(string)
	if name == "" {
		return to.ErrorResult(fmt.Errorf("name is required"))
	}
	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	att, err := svc.EditAttachment(ctx, owner, repo, int64(rid), int64(aid), name)
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(att)
}

func (h handlers) DeleteReleaseAttachmentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteReleaseAttachmentFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	rid, err := to.Float64(args["release_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("release_id: %w", err))
	}
	aid, err := to.Float64(args["attachment_id"])
	if err != nil {
		return to.ErrorResult(fmt.Errorf("attachment_id: %w", err))
	}
	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	if err := svc.DeleteAttachment(ctx, owner, repo, int64(rid), int64(aid)); err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(map[string]string{"status": "deleted"})
}
