// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"

	apprepo "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/repo"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/decode"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/params"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/to"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	GetFileToolName    = "get_file_content"
	CreateFileToolName = "create_file"
	UpdateFileToolName = "update_file"
	DeleteFileToolName = "delete_file"
)

var (
	GetFileContentTool = mcp.NewTool(
		GetFileToolName,
		mcp.WithDescription("Get file content as plain text by default. Set `with_metadata=true` for binary files, or when you need the SHA/encoding/links from the full `ContentsResponse` (e.g. before a follow-up `update_file` call). Optional `start_line` and `end_line` request a 1-indexed inclusive line range; out-of-range values clamp to the file extent. Range parameters are ignored when `with_metadata=true`."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("ref", mcp.Required(), mcp.Description(params.Ref)),
		mcp.WithString("filePath", mcp.Required(), mcp.Description(params.FilePath)),
		mcp.WithBoolean("with_metadata", mcp.Description("Return the full ContentsResponse (sha, encoding, links, type, size, base64 content) instead of plain text.")),
		mcp.WithNumber("start_line", mcp.Description("Optional 1-indexed first line of the slice (inclusive). Defaults to 1 when only end_line is set.")),
		mcp.WithNumber("end_line", mcp.Description("Optional 1-indexed last line of the slice (inclusive). Defaults to the file's last line when only start_line is set.")),
	)

	CreateFileTool = mcp.NewTool(
		CreateFileToolName,
		mcp.WithDescription("Create file. Supply exactly one of `content` (plain text) or `content_base64` (strict standard Base64 representing exact bytes)."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("filePath", mcp.Required(), mcp.Description(params.FilePath)),
		mcp.WithString("content", mcp.Description("Plain-text content. Exactly one of content/content_base64 is required; an explicit empty string is valid.")),
		mcp.WithString("content_base64", mcp.Description("Strict RFC 4648 standard Base64 for the exact file bytes. Exactly one of content/content_base64 is required; an explicit empty string represents zero bytes.")),
		mcp.WithString("message", mcp.Required(), mcp.Description(params.Message)),
		mcp.WithString("branch_name", mcp.Required(), mcp.Description(params.BranchName)),
		mcp.WithString("new_branch_name", mcp.Description(params.NewBranchName)),
	)

	UpdateFileTool = mcp.NewTool(
		UpdateFileToolName,
		mcp.WithDescription("Update file. Supply exactly one of `content` (plain text) or `content_base64` (strict standard Base64 representing exact bytes)."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("filePath", mcp.Required(), mcp.Description(params.FilePath)),
		mcp.WithString("content", mcp.Description("Plain-text content. Exactly one of content/content_base64 is required; an explicit empty string is valid.")),
		mcp.WithString("content_base64", mcp.Description("Strict RFC 4648 standard Base64 for the exact file bytes. Exactly one of content/content_base64 is required; an explicit empty string represents zero bytes.")),
		mcp.WithString("message", mcp.Required(), mcp.Description(params.Message)),
		mcp.WithString("branch_name", mcp.Required(), mcp.Description(params.BranchName)),
		mcp.WithString("sha", mcp.Required(), mcp.Description(params.SHA)),
		mcp.WithString("new_branch_name", mcp.Description(params.NewBranchName)),
	)

	DeleteFileTool = mcp.NewTool(
		DeleteFileToolName,
		mcp.WithDescription("Delete file"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("filePath", mcp.Required(), mcp.Description(params.FilePath)),
		mcp.WithString("message", mcp.Required(), mcp.Description(params.Message)),
		mcp.WithString("branch_name", mcp.Required(), mcp.Description(params.BranchName)),
		mcp.WithString("sha", mcp.Required(), mcp.Description(params.SHA)),
		mcp.WithString("new_branch_name", mcp.Description(params.NewBranchName)),
	)
)

// Argument structs for the file tools. The json names are the MCP arguments;
// mcptest.ArgsMatchSchema keeps them in agreement with the tool definitions.

type getFileContentArgs struct {
	Owner        *string `json:"owner" required:"true"`
	Repo         *string `json:"repo" required:"true"`
	Ref          string  `json:"ref" required:"true"`
	FilePath     *string `json:"filePath" required:"true"`
	WithMetadata bool    `json:"with_metadata"`
	StartLine    float64 `json:"start_line"`
	EndLine      float64 `json:"end_line"`
}

type createFileArgs struct {
	Owner         string `json:"owner" required:"true"`
	Repo          string `json:"repo" required:"true"`
	FilePath      string `json:"filePath" required:"true"`
	Message       string `json:"message" required:"true"`
	BranchName    string `json:"branch_name" required:"true"`
	NewBranchName string `json:"new_branch_name"`
	// Content and ContentBase64 are read by selectRepositoryWriteContent from
	// the raw arguments, which need their presence; they are declared so the
	// struct matches the schema.
	Content       *string `json:"content"`
	ContentBase64 *string `json:"content_base64"`
}

type updateFileArgs struct {
	Owner         string `json:"owner" required:"true"`
	Repo          string `json:"repo" required:"true"`
	FilePath      string `json:"filePath" required:"true"`
	Message       string `json:"message" required:"true"`
	BranchName    string `json:"branch_name" required:"true"`
	SHA           string `json:"sha" required:"true"`
	NewBranchName string `json:"new_branch_name"`
	// Content and ContentBase64 are read by selectRepositoryWriteContent from
	// the raw arguments, which need their presence; they are declared so the
	// struct matches the schema.
	Content       *string `json:"content"`
	ContentBase64 *string `json:"content_base64"`
}

type deleteFileArgs struct {
	Owner         *string `json:"owner" required:"true"`
	Repo          *string `json:"repo" required:"true"`
	FilePath      *string `json:"filePath" required:"true"`
	Message       string  `json:"message" required:"true"`
	BranchName    string  `json:"branch_name" required:"true"`
	SHA           *string `json:"sha" required:"true"`
	NewBranchName string  `json:"new_branch_name"`
}

func (h handlers) GetFileContentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetFileFn")
	var a getFileContentArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	owner, err := need("owner", a.Owner)
	if err != nil {
		return to.ErrorResult(err)
	}
	repo, err := need("repo", a.Repo)
	if err != nil {
		return to.ErrorResult(err)
	}
	filePath, err := need("filePath", a.FilePath)
	if err != nil {
		return to.ErrorResult(err)
	}
	svc, err := h.gitService(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	if a.WithMetadata {
		return gitText(svc.GetFileMetadata(ctx, owner, repo, a.Ref, filePath))
	}
	return gitText(svc.GetFileText(ctx, owner, repo, a.Ref, filePath, apprepo.LineRange{Start: int(a.StartLine), End: int(a.EndLine)}))
}

func (h handlers) CreateFileFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateFileFn")
	var a createFileArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	content, err := selectRepositoryWriteContent(req.GetArguments())
	if err != nil {
		return to.ErrorResult(err)
	}
	svc, err := h.gitService(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	return gitText(svc.CreateFile(ctx, a.Owner, a.Repo, a.FilePath, apprepo.FileWriteInput{
		Message: a.Message, Branch: a.BranchName, NewBranch: a.NewBranchName, ContentBase64: content,
	}))
}

func (h handlers) UpdateFileFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called UpdateFileFn")
	var a updateFileArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	content, err := selectRepositoryWriteContent(req.GetArguments())
	if err != nil {
		return to.ErrorResult(err)
	}
	svc, err := h.gitService(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	return gitText(svc.UpdateFile(ctx, a.Owner, a.Repo, a.FilePath, apprepo.FileWriteInput{
		Message: a.Message, Branch: a.BranchName, NewBranch: a.NewBranchName, ContentBase64: content, SHA: a.SHA,
	}))
}

func (h handlers) DeleteFileFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteFileFn")
	var a deleteFileArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	owner, err := need("owner", a.Owner)
	if err != nil {
		return to.ErrorResult(err)
	}
	repo, err := need("repo", a.Repo)
	if err != nil {
		return to.ErrorResult(err)
	}
	filePath, err := need("filePath", a.FilePath)
	if err != nil {
		return to.ErrorResult(err)
	}
	sha, err := need("sha", a.SHA)
	if err != nil {
		return to.ErrorResult(err)
	}
	svc, err := h.gitService(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	if err := svc.DeleteFile(ctx, owner, repo, filePath, apprepo.FileDeleteInput{
		Message: a.Message, Branch: a.BranchName, NewBranch: a.NewBranchName, SHA: sha,
	}); err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult("Delete file success")
}
