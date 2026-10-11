// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"
	"fmt"
	"strings"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/params"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/to"

	"github.com/mark3labs/mcp-go/mcp"
)

const ChangeFilesToolName = "change_files"

var ChangeFilesTool = mcp.NewTool(
	ChangeFilesToolName,
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

func (h handlers) ChangeFilesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ChangeFilesFn")

	owner, repo, options, err := parseChangeFilesArguments(req.GetArguments())
	if err != nil {
		return to.ErrorResult(err)
	}

	port, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	response, err := port.ChangeFiles(ctx, owner, repo, options)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("change files error: %w", err))
	}
	return to.TextResult(response)
}

func parseChangeFilesArguments(args map[string]any) (string, string, forge.ChangeFilesOptions, error) {
	owner, err := requiredNonBlankString(args, "owner")
	if err != nil {
		return "", "", forge.ChangeFilesOptions{}, err
	}
	repo, err := requiredNonBlankString(args, "repo")
	if err != nil {
		return "", "", forge.ChangeFilesOptions{}, err
	}
	message, err := requiredNonBlankString(args, "message")
	if err != nil {
		return "", "", forge.ChangeFilesOptions{}, err
	}
	branchName, err := requiredNonBlankString(args, "branch_name")
	if err != nil {
		return "", "", forge.ChangeFilesOptions{}, err
	}

	newBranchName := ""
	if raw, ok := args["new_branch_name"]; ok {
		value, ok := raw.(string)
		if !ok {
			return "", "", forge.ChangeFilesOptions{}, fmt.Errorf("new_branch_name must be a string")
		}
		newBranchName = value
	}

	rawFiles, ok := args["files"].([]any)
	if !ok || len(rawFiles) == 0 {
		return "", "", forge.ChangeFilesOptions{}, fmt.Errorf("files must contain at least one operation")
	}

	operations := make([]forge.ChangeFileOperation, 0, len(rawFiles))
	seenPaths := make(map[string]struct{}, len(rawFiles))
	for index, raw := range rawFiles {
		item, ok := raw.(map[string]any)
		if !ok {
			return "", "", forge.ChangeFilesOptions{}, fmt.Errorf("files[%d] must be an object", index)
		}

		operation, err := requiredNonBlankString(item, "operation")
		if err != nil {
			return "", "", forge.ChangeFilesOptions{}, fmt.Errorf("files[%d]: %w", index, err)
		}
		path, err := requiredNonBlankString(item, "path")
		if err != nil {
			return "", "", forge.ChangeFilesOptions{}, fmt.Errorf("files[%d]: %w", index, err)
		}
		if _, duplicate := seenPaths[path]; duplicate {
			return "", "", forge.ChangeFilesOptions{}, fmt.Errorf("files[%d]: duplicate target path %q", index, path)
		}
		seenPaths[path] = struct{}{}

		sha, hasSHA, err := optionalString(item, "sha")
		if err != nil {
			return "", "", forge.ChangeFilesOptions{}, fmt.Errorf("files[%d]: %w", index, err)
		}

		entry := forge.ChangeFileOperation{Operation: operation, Path: path}
		switch operation {
		case "create":
			if hasSHA {
				return "", "", forge.ChangeFilesOptions{}, fmt.Errorf("files[%d]: sha must be omitted for create", index)
			}
			encoded, err := selectRepositoryWriteContent(item)
			if err != nil {
				return "", "", forge.ChangeFilesOptions{}, fmt.Errorf("files[%d]: %w", index, err)
			}
			entry.Content = &encoded
		case "update":
			if !hasSHA || strings.TrimSpace(sha) == "" {
				return "", "", forge.ChangeFilesOptions{}, fmt.Errorf("files[%d]: sha is required for update", index)
			}
			encoded, err := selectRepositoryWriteContent(item)
			if err != nil {
				return "", "", forge.ChangeFilesOptions{}, fmt.Errorf("files[%d]: %w", index, err)
			}
			entry.Content = &encoded
			entry.SHA = sha
		case "delete":
			if err := rejectRepositoryWriteContent(item); err != nil {
				return "", "", forge.ChangeFilesOptions{}, fmt.Errorf("files[%d]: %w", index, err)
			}
			if !hasSHA || strings.TrimSpace(sha) == "" {
				return "", "", forge.ChangeFilesOptions{}, fmt.Errorf("files[%d]: sha is required for delete", index)
			}
			entry.SHA = sha
		default:
			return "", "", forge.ChangeFilesOptions{}, fmt.Errorf("files[%d]: unsupported operation %q", index, operation)
		}
		operations = append(operations, entry)
	}

	return owner, repo, forge.ChangeFilesOptions{
		BranchName:    branchName,
		NewBranchName: newBranchName,
		Message:       message,
		Files:         operations,
	}, nil
}

func requiredNonBlankString(values map[string]any, name string) (string, error) {
	raw, ok := values[name]
	if !ok {
		return "", fmt.Errorf("%s is required", name)
	}
	value, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("%s must be a string", name)
	}
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%s must not be empty", name)
	}
	return value, nil
}
