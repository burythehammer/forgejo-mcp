// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"
	"fmt"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/params"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/lines"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/to"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
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

func (h handlers) GetFileContentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetFileFn")
	args := req.GetArguments()
	owner, ok := args["owner"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("owner is required"))
	}
	repo, ok := args["repo"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("repo is required"))
	}
	ref, _ := args["ref"].(string)
	filePath, ok := args["filePath"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("filePath is required"))
	}
	withMetadata, _ := args["with_metadata"].(bool)

	if withMetadata {
		port, err := h.forge.For(ctx)
		if err != nil {
			return to.ErrorResult(err)
		}
		content, err := port.GetContents(ctx, owner, repo, ref, filePath)
		if err != nil {
			return to.ErrorResult(fmt.Errorf("get file err: %w", err))
		}
		return to.TextResult(content)
	}

	// Default: plain text via GetFile (SDK /raw/ endpoint, no base64).
	// GetFile returns []byte; binary files are returned as-is without detection.
	port, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	rawBytes, err := port.GetFile(ctx, owner, repo, ref, filePath)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get file err: %w", err))
	}

	startF, _ := to.Float64(args["start_line"])
	endF, _ := to.Float64(args["end_line"])
	if startF == 0 && endF == 0 {
		return to.TextResult(string(rawBytes))
	}

	sliced, err := SliceLines(string(rawBytes), int(startF), int(endF))
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(sliced)
}

// SliceLines returns the 1-indexed inclusive [start, end] line range
// of content. start=0 means "from line 1"; end=0 means "to the last
// line". Out-of-range values clamp; an inverted range after clamping
// is reported as an error.
func SliceLines(content string, start, end int) (string, error) {
	return lines.Slice(content, start, end)
}

func (h handlers) CreateFileFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateFileFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	filePath, _ := args["filePath"].(string)
	encodedContent, err := selectRepositoryWriteContent(args)
	if err != nil {
		return to.ErrorResult(err)
	}
	message, _ := args["message"].(string)
	branchName, _ := args["branch_name"].(string)
	newBranchName, ok := args["new_branch_name"].(string)
	if !ok || newBranchName == "" {
		newBranchName = ""
	}
	opt := forgejo_sdk.CreateFileOptions{
		FileOptions: forgejo_sdk.FileOptions{
			Message:       message,
			BranchName:    branchName,
			NewBranchName: newBranchName,
		},
		Content: encodedContent,
	}
	port, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	fileResp, err := port.CreateFile(ctx, owner, repo, filePath, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("create file error: %w", err))
	}
	return to.TextResult(fileResp)
}

func (h handlers) UpdateFileFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called UpdateFileFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	filePath, _ := args["filePath"].(string)
	encodedContent, err := selectRepositoryWriteContent(args)
	if err != nil {
		return to.ErrorResult(err)
	}
	message, _ := args["message"].(string)
	branchName, _ := args["branch_name"].(string)
	sha, _ := args["sha"].(string)
	newBranchName, ok := args["new_branch_name"].(string)
	if !ok || newBranchName == "" {
		newBranchName = ""
	}
	opt := forgejo_sdk.UpdateFileOptions{
		FileOptions: forgejo_sdk.FileOptions{
			Message:       message,
			BranchName:    branchName,
			NewBranchName: newBranchName,
		},
		SHA:     sha,
		Content: encodedContent,
	}
	port, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	fileResp, err := port.UpdateFile(ctx, owner, repo, filePath, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("update file error: %w", err))
	}
	return to.TextResult(fileResp)
}

func (h handlers) DeleteFileFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteFileFn")
	owner, ok := req.GetArguments()["owner"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("owner is required"))
	}
	repo, ok := req.GetArguments()["repo"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("repo is required"))
	}
	filePath, ok := req.GetArguments()["filePath"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("filePath is required"))
	}
	message, _ := req.GetArguments()["message"].(string)
	branchName, _ := req.GetArguments()["branch_name"].(string)
	sha, ok := req.GetArguments()["sha"].(string)
	if !ok {
		return to.ErrorResult(fmt.Errorf("sha is required"))
	}
	newBranchName, _ := req.GetArguments()["new_branch_name"].(string)
	opt := forgejo_sdk.DeleteFileOptions{
		FileOptions: forgejo_sdk.FileOptions{
			Message:       message,
			BranchName:    branchName,
			NewBranchName: newBranchName,
		},
		SHA: sha,
	}
	port, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	err = port.DeleteFile(ctx, owner, repo, filePath, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("delete file err: %w", err))
	}
	return to.TextResult("Delete file success")
}
