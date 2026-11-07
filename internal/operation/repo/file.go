// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/numarg"

	apprepo "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/repo"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/annotate"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/decode"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcpresult"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/params"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/pipeline"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/to"

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
		annotate.Read(),
		mcp.WithDescription("Get file content as plain text by default. Set `with_metadata=true` for binary files, or when you need the SHA/encoding/links from the full `ContentsResponse` (e.g. before a follow-up `update_file` call). Optional `start_line` and `end_line` request a 1-indexed inclusive line range; out-of-range values clamp to the file extent. With `with_metadata=true` the range replaces the content with that slice and keeps the metadata. A whole file, or a range, over 1 MiB of text is refused with a message asking for a narrower `start_line`/`end_line`."),
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
		annotate.Write(),
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
		annotate.Write(),
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
		annotate.Destructive(),
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
	// Content and ContentBase64 are the two content selectors; selection needs
	// whether each was sent, so they are decode.Arg.
	Content       decode.Arg `json:"content" schema:"string"`
	ContentBase64 decode.Arg `json:"content_base64" schema:"string"`
}

type updateFileArgs struct {
	Owner         string `json:"owner" required:"true"`
	Repo          string `json:"repo" required:"true"`
	FilePath      string `json:"filePath" required:"true"`
	Message       string `json:"message" required:"true"`
	BranchName    string `json:"branch_name" required:"true"`
	SHA           string `json:"sha" required:"true"`
	NewBranchName string `json:"new_branch_name"`
	// Content and ContentBase64 are the two content selectors; selection needs
	// whether each was sent, so they are decode.Arg.
	Content       decode.Arg `json:"content" schema:"string"`
	ContentBase64 decode.Arg `json:"content_base64" schema:"string"`
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

// target is the repository and file path a file read names. A missing one is
// refused in Validate, before the credential lookup.
func (a getFileContentArgs) target() (owner, repo, filePath string, err error) {
	if owner, err = need("owner", a.Owner); err != nil {
		return "", "", "", err
	}
	if repo, err = need("repo", a.Repo); err != nil {
		return "", "", "", err
	}
	if filePath, err = need("filePath", a.FilePath); err != nil {
		return "", "", "", err
	}
	return owner, repo, filePath, nil
}

// Validate refuses a file read that names no repository or file path.
func (a getFileContentArgs) Validate() error {
	_, _, _, err := a.target()
	return err
}

func (h handlers) GetFileContentFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return pipeline.Handle(h.gitService, getFileContent)(ctx, req)
}

func getFileContent(ctx context.Context, svc *apprepo.GitService, a getFileContentArgs) (*mcp.CallToolResult, error) {
	owner, repo, filePath, err := a.target()
	if err != nil {
		return mcpresult.Fail(err)
	}
	lines := apprepo.LineRange{Start: int(numarg.Truncate(a.StartLine)), End: int(numarg.Truncate(a.EndLine))}
	if a.WithMetadata {
		return gitText(svc.GetFileMetadata(ctx, owner, repo, a.Ref, filePath, lines))
	}
	return gitText(svc.GetFileText(ctx, owner, repo, a.Ref, filePath, lines))
}

func (h handlers) CreateFileFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return pipeline.Handle(h.gitService, createFile)(ctx, req)
}

func createFile(ctx context.Context, svc *apprepo.GitService, a createFileArgs) (*mcp.CallToolResult, error) {
	content, err := selectRepositoryWriteContent(a.Content, a.ContentBase64)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return gitText(svc.CreateFile(ctx, a.Owner, a.Repo, a.FilePath, apprepo.FileWriteInput{
		Message: a.Message, Branch: a.BranchName, NewBranch: a.NewBranchName, ContentBase64: content,
	}))
}

func (h handlers) UpdateFileFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return pipeline.Handle(h.gitService, updateFile)(ctx, req)
}

func updateFile(ctx context.Context, svc *apprepo.GitService, a updateFileArgs) (*mcp.CallToolResult, error) {
	content, err := selectRepositoryWriteContent(a.Content, a.ContentBase64)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return gitText(svc.UpdateFile(ctx, a.Owner, a.Repo, a.FilePath, apprepo.FileWriteInput{
		Message: a.Message, Branch: a.BranchName, NewBranch: a.NewBranchName, ContentBase64: content, SHA: a.SHA,
	}))
}

// target is the repository, file path and blob SHA a delete names. A missing
// one is refused in Validate, before the credential lookup.
func (a deleteFileArgs) target() (owner, repo, filePath, sha string, err error) {
	if owner, err = need("owner", a.Owner); err != nil {
		return "", "", "", "", err
	}
	if repo, err = need("repo", a.Repo); err != nil {
		return "", "", "", "", err
	}
	if filePath, err = need("filePath", a.FilePath); err != nil {
		return "", "", "", "", err
	}
	if sha, err = need("sha", a.SHA); err != nil {
		return "", "", "", "", err
	}
	return owner, repo, filePath, sha, nil
}

// Validate refuses a delete that names no repository, file path or SHA.
func (a deleteFileArgs) Validate() error {
	_, _, _, _, err := a.target()
	return err
}

func (h handlers) DeleteFileFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return pipeline.Handle(h.gitService, deleteFile)(ctx, req)
}

func deleteFile(ctx context.Context, svc *apprepo.GitService, a deleteFileArgs) (*mcp.CallToolResult, error) {
	owner, repo, filePath, sha, err := a.target()
	if err != nil {
		return mcpresult.Fail(err)
	}
	if err := svc.DeleteFile(ctx, owner, repo, filePath, apprepo.FileDeleteInput{
		Message: a.Message, Branch: a.BranchName, NewBranch: a.NewBranchName, SHA: sha,
	}); err != nil {
		return mcpresult.Fail(err)
	}
	return to.TextResult("Delete file success")
}

// Validate refuses a create that names no repository, path, message or branch, or whose content
// selectors are not exactly one. The write runs only after the credential lookup.
func (a createFileArgs) Validate() error {
	if err := firstError(
		requireString("owner", a.Owner), requireString("repo", a.Repo), requireString("filePath", a.FilePath),
		requireString("message", a.Message), requireString("branch_name", a.BranchName),
	); err != nil {
		return err
	}
	_, err := selectRepositoryWriteContent(a.Content, a.ContentBase64)
	return err
}

// Validate refuses an update that names no repository, path, message or branch, or whose content
// selectors are not exactly one. A missing SHA is not refused: the update takes the current one.
func (a updateFileArgs) Validate() error {
	if err := firstError(
		requireString("owner", a.Owner), requireString("repo", a.Repo), requireString("filePath", a.FilePath),
		requireString("message", a.Message), requireString("branch_name", a.BranchName),
	); err != nil {
		return err
	}
	_, err := selectRepositoryWriteContent(a.Content, a.ContentBase64)
	return err
}
