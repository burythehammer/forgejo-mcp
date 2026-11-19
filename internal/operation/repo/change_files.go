// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"
	"fmt"
	"strings"

	apprepo "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/repo"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/annotate"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/decode"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/mcpresult"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/params"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/pipeline"

	"github.com/mark3labs/mcp-go/mcp"
)

const ChangeFilesToolName = "change_files"

var ChangeFilesTool = mcp.NewTool(
	ChangeFilesToolName,
	annotate.Write(),
	mcp.WithDescription("Atomically create, update, and delete multiple repository files in one native Forgejo commit. The complete batch is validated before the write; create/update operations require exactly one of `content` or `content_base64`, and update/delete operations require the current file SHA. Existing single-file tools remain available."),
	mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
	mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
	mcp.WithString("message", mcp.Required(), mcp.Description(params.Message)),
	mcp.WithString("branch_name", mcp.Required(), mcp.Description(params.BranchName)),
	mcp.WithString("new_branch_name", mcp.Description(params.NewBranchName)),
	mcp.WithArray(
		"files",
		mcp.Required(),
		mcp.MinItems(1),
		mcp.Description("File operations applied atomically in one Forgejo commit. Create/update require exactly one of plain-text `content` or strict standard Base64 `content_base64`; delete forbids both representations."),
		mcp.Items(map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"required":             []string{"operation", "path"},
			"properties": map[string]any{
				"operation": map[string]any{
					"type":        "string",
					"enum":        []string{"create", "update", "delete"},
					"description": "Repository file operation.",
				},
				"path": map[string]any{
					"type":        "string",
					"minLength":   1,
					"description": "Repository-relative target path.",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "Plain-text content. Create/update require exactly one of content/content_base64; delete forbids both. An explicit empty string is valid content.",
				},
				"content_base64": map[string]any{
					"type":        "string",
					"description": "Strict RFC 4648 standard Base64 for the exact file bytes. Create/update require exactly one of content/content_base64; delete forbids both. An explicit empty string represents zero bytes.",
				},
				"sha": map[string]any{
					"type":        "string",
					"description": "Current file SHA. Required for update/delete and omitted for create.",
				},
			},
		}),
	),
)

// Argument structs for the batch: the top level, and one entry of `files`.
// `json` names the MCP argument; mcptest.ArgsMatchSchema keeps them in
// agreement with the schema. A decode.Arg records whether the argument was
// sent, null included, because the batch's error texts depend on that.
// Files is read as a plain array and each entry is then decoded on its own,
// so an entry that is not an object gets the tool's own message.

type changeFilesArgs struct {
	Owner         decode.Arg `json:"owner" required:"true" schema:"string"`
	Repo          decode.Arg `json:"repo" required:"true" schema:"string"`
	Message       decode.Arg `json:"message" required:"true" schema:"string"`
	BranchName    decode.Arg `json:"branch_name" required:"true" schema:"string"`
	NewBranchName decode.Arg `json:"new_branch_name" schema:"string"`
	Files         any        `json:"files" required:"true" schema:"array"`
}

type changeFileArgs struct {
	Operation     decode.Arg `json:"operation" required:"true" schema:"string"`
	Path          decode.Arg `json:"path" required:"true" schema:"string"`
	Content       decode.Arg `json:"content" schema:"string"`
	ContentBase64 decode.Arg `json:"content_base64" schema:"string"`
	SHA           decode.Arg `json:"sha" schema:"string"`
}

// Validate refuses a batch that parseChangeFiles rejects. The batch's errors are
// the tool's own, so they are returned as the refusal, before the credential lookup.
func (a changeFilesArgs) Validate() error {
	_, _, _, err := parseChangeFiles(a)
	return err
}

func (h handlers) ChangeFilesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return pipeline.Handle(h.gitService, changeFiles)(ctx, req)
}

func changeFiles(ctx context.Context, svc *apprepo.GitService, a changeFilesArgs) (*mcp.CallToolResult, error) {
	owner, repo, in, err := parseChangeFiles(a)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return gitText(svc.ChangeFiles(ctx, owner, repo, in))
}

func parseChangeFiles(a changeFilesArgs) (string, string, apprepo.ChangeFilesInput, error) {
	owner, err := requiredNonBlank("owner", a.Owner)
	if err != nil {
		return "", "", apprepo.ChangeFilesInput{}, err
	}
	repo, err := requiredNonBlank("repo", a.Repo)
	if err != nil {
		return "", "", apprepo.ChangeFilesInput{}, err
	}
	message, err := requiredNonBlank("message", a.Message)
	if err != nil {
		return "", "", apprepo.ChangeFilesInput{}, err
	}
	branchName, err := requiredNonBlank("branch_name", a.BranchName)
	if err != nil {
		return "", "", apprepo.ChangeFilesInput{}, err
	}

	newBranchName, _, err := a.NewBranchName.String("new_branch_name")
	if err != nil {
		return "", "", apprepo.ChangeFilesInput{}, err
	}

	rawFiles, ok := a.Files.([]any)
	if !ok || len(rawFiles) == 0 {
		return "", "", apprepo.ChangeFilesInput{}, fmt.Errorf("files must contain at least one operation")
	}

	operations := make([]apprepo.ChangeFile, 0, len(rawFiles))
	seenPaths := make(map[string]struct{}, len(rawFiles))
	for index, raw := range rawFiles {
		item, ok := raw.(map[string]any)
		if !ok {
			return "", "", apprepo.ChangeFilesInput{}, fmt.Errorf("files[%d] must be an object", index)
		}
		var f changeFileArgs
		if err := decode.Map(item, &f); err != nil {
			return "", "", apprepo.ChangeFilesInput{}, fmt.Errorf("files[%d]: %w", index, err)
		}

		operation, err := requiredNonBlank("operation", f.Operation)
		if err != nil {
			return "", "", apprepo.ChangeFilesInput{}, fmt.Errorf("files[%d]: %w", index, err)
		}
		path, err := requiredNonBlank("path", f.Path)
		if err != nil {
			return "", "", apprepo.ChangeFilesInput{}, fmt.Errorf("files[%d]: %w", index, err)
		}
		if _, duplicate := seenPaths[path]; duplicate {
			return "", "", apprepo.ChangeFilesInput{}, fmt.Errorf("files[%d]: duplicate target path %q", index, path)
		}
		seenPaths[path] = struct{}{}

		sha, hasSHA, err := f.SHA.String("sha")
		if err != nil {
			return "", "", apprepo.ChangeFilesInput{}, fmt.Errorf("files[%d]: %w", index, err)
		}

		entry := apprepo.ChangeFile{Operation: operation, Path: path}
		switch operation {
		case "create":
			if hasSHA {
				return "", "", apprepo.ChangeFilesInput{}, fmt.Errorf("files[%d]: sha must be omitted for create", index)
			}
			encoded, err := selectRepositoryWriteContent(f.Content, f.ContentBase64)
			if err != nil {
				return "", "", apprepo.ChangeFilesInput{}, fmt.Errorf("files[%d]: %w", index, err)
			}
			entry.Content = &encoded
		case "update":
			if !hasSHA || strings.TrimSpace(sha) == "" {
				return "", "", apprepo.ChangeFilesInput{}, fmt.Errorf("files[%d]: sha is required for update", index)
			}
			encoded, err := selectRepositoryWriteContent(f.Content, f.ContentBase64)
			if err != nil {
				return "", "", apprepo.ChangeFilesInput{}, fmt.Errorf("files[%d]: %w", index, err)
			}
			entry.Content = &encoded
			entry.SHA = sha
		case "delete":
			if err := rejectRepositoryWriteContent(f.Content, f.ContentBase64); err != nil {
				return "", "", apprepo.ChangeFilesInput{}, fmt.Errorf("files[%d]: %w", index, err)
			}
			if !hasSHA || strings.TrimSpace(sha) == "" {
				return "", "", apprepo.ChangeFilesInput{}, fmt.Errorf("files[%d]: sha is required for delete", index)
			}
			entry.SHA = sha
		default:
			return "", "", apprepo.ChangeFilesInput{}, fmt.Errorf("files[%d]: unsupported operation %q", index, operation)
		}
		operations = append(operations, entry)
	}

	return owner, repo, apprepo.ChangeFilesInput{
		Branch:    branchName,
		NewBranch: newBranchName,
		Message:   message,
		Files:     operations,
	}, nil
}

// requiredNonBlank reads a required string: absent is "<name> is required", a
// value that is not a string (null included) is "<name> must be a string", and
// a blank one is "<name> must not be empty".
func requiredNonBlank(name string, a decode.Arg) (string, error) {
	if !a.Present() {
		return "", fmt.Errorf("%s is required", name)
	}
	value, _, err := a.String(name)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%s must not be empty", name)
	}
	return value, nil
}
