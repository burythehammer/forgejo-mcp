// SPDX-License-Identifier: GPL-3.0-or-later

package forge

import (
	"context"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// RepoFiles is the repository-content half of the repo domain: files and
// directories at a ref, the git tree, writing files, commits, and commit
// statuses. Repository administration (branches, topics, editing, overview)
// is a separate port.
//
// Failures from Forgejo are *StatusError. Content passed to the write methods
// is already Base64, as the Forgejo contents API expects it.
type RepoFiles interface {
	// GetFile returns the raw bytes of a file, as the /raw/ endpoint serves them.
	GetFile(ctx context.Context, owner, repo, ref, path string) ([]byte, error)
	// GetContents returns a file's metadata and Base64 content.
	GetContents(ctx context.Context, owner, repo, ref, path string) (*forgejo_sdk.ContentsResponse, error)
	// ListContents returns the entries of one directory level; an empty path is the root.
	ListContents(ctx context.Context, owner, repo, ref, path string) ([]*forgejo_sdk.ContentsResponse, error)
	GetTree(ctx context.Context, owner, repo, ref string, opt forgejo_sdk.GetTreesOptions) (*forgejo_sdk.GitTreeResponse, error)

	CreateFile(ctx context.Context, owner, repo, path string, opt forgejo_sdk.CreateFileOptions) (*forgejo_sdk.FileResponse, error)
	UpdateFile(ctx context.Context, owner, repo, path string, opt forgejo_sdk.UpdateFileOptions) (*forgejo_sdk.FileResponse, error)
	DeleteFile(ctx context.Context, owner, repo, path string, opt forgejo_sdk.DeleteFileOptions) error
	// ChangeFiles applies several file operations in one commit, atomically,
	// and returns Forgejo's answer (commit and per-file results) as decoded JSON.
	ChangeFiles(ctx context.Context, owner, repo string, opt ChangeFilesOptions) (map[string]any, error)

	GetCommit(ctx context.Context, owner, repo, sha string) (*forgejo_sdk.Commit, error)
	// ListCommits lists commits newest first; opt.Path keeps only commits
	// touching that path.
	ListCommits(ctx context.Context, owner, repo string, opt forgejo_sdk.ListCommitOptions) ([]*forgejo_sdk.Commit, error)

	ListCommitStatuses(ctx context.Context, owner, repo, sha string, opt forgejo_sdk.ListStatusesOption) (CommitStatuses, error)
}

// CommitStatuses is one page of a commit's per-context statuses.
type CommitStatuses struct {
	Statuses []*forgejo_sdk.Status
	// TotalCount is Forgejo's X-Total-Count; nil when it did not report one.
	TotalCount *int
}

// ChangeFileOperation is one entry of a ChangeFiles batch. Operation is
// "create", "update" or "delete"; Content is Base64 and nil for delete; SHA is
// the current file SHA, required for update and delete.
type ChangeFileOperation struct {
	Operation string  `json:"operation"`
	Path      string  `json:"path"`
	Content   *string `json:"content,omitempty"`
	SHA       string  `json:"sha,omitempty"`
}

// ChangeFilesOptions is a ChangeFiles batch and its commit.
type ChangeFilesOptions struct {
	BranchName    string                `json:"branch"`
	NewBranchName string                `json:"new_branch,omitempty"`
	Message       string                `json:"message"`
	Files         []ChangeFileOperation `json:"files"`
}
