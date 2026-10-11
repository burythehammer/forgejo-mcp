// SPDX-License-Identifier: GPL-3.0-or-later

// Package repo holds the repository use cases behind the repo tools. Like all
// of app/ it is MCP-free and SDK-free (scripts/ci/check-app-boundary.sh): use
// cases take typed inputs, reach Forgejo only through GitPort, and return
// repo-owned types from app/model for an MCP adapter in operation/repo to
// render.
package repo

import (
	"context"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
)

// GitPort is the repository branch, commit, contents and file operations the
// use cases need, in repo-owned types. pkg/forge/repogitadapter implements it
// over forge.RepoAdmin and forge.RepoFiles. Failures from Forgejo come back as
// ordinary errors for the use case to wrap.
type GitPort interface {
	ListBranches(ctx context.Context, owner, repo string, p GitPage) ([]*model.Branch, error)
	CreateBranch(ctx context.Context, owner, repo string, in CreateBranchInput) error
	// DeleteBranch reports whether Forgejo deleted the branch (HTTP 204).
	DeleteBranch(ctx context.Context, owner, repo, branch string) (bool, error)

	ListCommits(ctx context.Context, owner, repo string, q ListCommitsInput) ([]*model.Commit, error)

	ListContents(ctx context.Context, owner, repo, ref, path string) ([]*model.ContentsResponse, error)
	GetTree(ctx context.Context, owner, repo, ref string, q TreeInput) (*model.GitTreeResponse, error)
	GetFile(ctx context.Context, owner, repo, ref, path string) ([]byte, error)
	GetContents(ctx context.Context, owner, repo, ref, path string) (*model.ContentsResponse, error)
	CreateFile(ctx context.Context, owner, repo, path string, in FileWriteInput) (*model.FileResponse, error)
	UpdateFile(ctx context.Context, owner, repo, path string, in FileWriteInput) (*model.FileResponse, error)
	DeleteFile(ctx context.Context, owner, repo, path string, in FileDeleteInput) error
	ChangeFiles(ctx context.Context, owner, repo string, in ChangeFilesInput) (map[string]any, error)
}

// GitPage selects a page of a list; zero values are left to Forgejo.
type GitPage struct{ Page, Limit int }

// CreateBranchInput names a new branch and the branch it starts from.
type CreateBranchInput struct{ Branch, OldBranch string }

// ListCommitsInput filters and pages a commit listing.
type ListCommitsInput struct {
	Path, SHA string
	Page      GitPage
}

// TreeInput selects a tree, and whether it is returned recursively.
type TreeInput struct {
	Recursive bool
	Page      GitPage
}

// FileWriteInput is a create or update of one file. ContentBase64 is the
// exact file content; SHA is the current file SHA, set for an update only.
type FileWriteInput struct {
	Message, Branch, NewBranch string
	ContentBase64              string
	SHA                        string
}

// FileDeleteInput is the deletion of one file at its current SHA.
type FileDeleteInput struct {
	Message, Branch, NewBranch string
	SHA                        string
}

// ChangeFilesInput is a batch of file operations applied in one commit.
type ChangeFilesInput struct {
	Message, Branch, NewBranch string
	Files                      []ChangeFile
}

// ChangeFile is one operation of a batch. Operation is "create", "update" or
// "delete"; Content is the Base64 content and nil for a delete; SHA is the
// current file SHA, required for an update and a delete.
type ChangeFile struct {
	Operation string
	Path      string
	Content   *string
	SHA       string
}
