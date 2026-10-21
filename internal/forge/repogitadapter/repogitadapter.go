// SPDX-License-Identifier: GPL-3.0-or-later

// Package repogitadapter implements the repo use cases' GitPort (internal/app/repo.GitPort)
// over the Forgejo ports for branches, commits and files (forge.RepoAdmin and
// forge.RepoFiles). It is the one place that translates between repo-owned
// types and SDK types for these operations: inputs become SDK options, SDK
// results become internal/app/model values. Results are converted through their JSON
// form, which is exactly what a tool renders, so a field the repo-owned type
// shares by JSON name cannot drift in value.
package repogitadapter

import (
	"context"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	apprepo "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/repo"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/bridge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// Ports is what the adapter reads from: the repository administration and file
// domains of the Forgejo port. A forge.Forgejo satisfies it.
type Ports interface {
	forge.RepoAdmin
	forge.RepoFiles
	forge.Ceilings
}

// New returns the use cases' port over p.
func New(p Ports) apprepo.GitPort { return adapter{p: p} }

type adapter struct{ p Ports }

// MaxResponseItems reads the instance's pagination ceiling through the port.
func (a adapter) MaxResponseItems(ctx context.Context) (int, bool) {
	return a.p.MaxResponseItems(ctx)
}

var _ apprepo.GitPort = adapter{}

func listOptions(p apprepo.GitPage) forgejo_sdk.ListOptions {
	return forgejo_sdk.ListOptions{Page: p.Page, PageSize: p.Limit}
}

func fileOptions(message, branch, newBranch string) forgejo_sdk.FileOptions {
	return forgejo_sdk.FileOptions{Message: message, BranchName: branch, NewBranchName: newBranch}
}

func (a adapter) ListBranches(ctx context.Context, owner, repo string, p apprepo.GitPage) ([]*model.Branch, error) {
	bs, err := a.p.ListBranches(ctx, owner, repo, forgejo_sdk.ListRepoBranchesOptions{ListOptions: listOptions(p)})
	return bridge.Convert[[]*model.Branch](bs, err)
}

func (a adapter) CreateBranch(ctx context.Context, owner, repo string, in apprepo.CreateBranchInput) error {
	_, err := a.p.CreateBranch(ctx, owner, repo, forgejo_sdk.CreateBranchOption{BranchName: in.Branch, OldBranchName: in.OldBranch})
	return err
}

func (a adapter) DeleteBranch(ctx context.Context, owner, repo, branch string) (bool, error) {
	return a.p.DeleteBranch(ctx, owner, repo, branch)
}

func (a adapter) GetCommit(ctx context.Context, owner, repo, sha string) (*model.Commit, error) {
	c, err := a.p.GetCommit(ctx, owner, repo, sha)
	return bridge.Convert[*model.Commit](c, err)
}

func (a adapter) ListCommits(ctx context.Context, owner, repo string, q apprepo.ListCommitsInput) ([]*model.Commit, error) {
	cs, err := a.p.ListCommits(ctx, owner, repo, forgejo_sdk.ListCommitOptions{Path: q.Path, SHA: q.SHA, ListOptions: listOptions(q.Page)})
	return bridge.Convert[[]*model.Commit](cs, err)
}

func (a adapter) ListContents(ctx context.Context, owner, repo, ref, path string) ([]*model.ContentsResponse, error) {
	cs, err := a.p.ListContents(ctx, owner, repo, ref, path)
	return bridge.Convert[[]*model.ContentsResponse](cs, err)
}

func (a adapter) GetTree(ctx context.Context, owner, repo, ref string, q apprepo.TreeInput) (*model.GitTreeResponse, error) {
	t, err := a.p.GetTree(ctx, owner, repo, ref, forgejo_sdk.GetTreesOptions{Recursive: q.Recursive, ListOptions: listOptions(q.Page)})
	return bridge.Convert[*model.GitTreeResponse](t, err)
}

func (a adapter) GetFile(ctx context.Context, owner, repo, ref, path string) ([]byte, error) {
	return a.p.GetFile(ctx, owner, repo, ref, path)
}

func (a adapter) GetContents(ctx context.Context, owner, repo, ref, path string) (*model.ContentsResponse, error) {
	c, err := a.p.GetContents(ctx, owner, repo, ref, path)
	return bridge.Convert[*model.ContentsResponse](c, err)
}

func (a adapter) CreateFile(ctx context.Context, owner, repo, path string, in apprepo.FileWriteInput) (*model.FileResponse, error) {
	f, err := a.p.CreateFile(ctx, owner, repo, path, forgejo_sdk.CreateFileOptions{
		FileOptions: fileOptions(in.Message, in.Branch, in.NewBranch),
		Content:     in.ContentBase64,
	})
	return bridge.Convert[*model.FileResponse](f, err)
}

func (a adapter) UpdateFile(ctx context.Context, owner, repo, path string, in apprepo.FileWriteInput) (*model.FileResponse, error) {
	f, err := a.p.UpdateFile(ctx, owner, repo, path, forgejo_sdk.UpdateFileOptions{
		FileOptions: fileOptions(in.Message, in.Branch, in.NewBranch),
		SHA:         in.SHA,
		Content:     in.ContentBase64,
	})
	return bridge.Convert[*model.FileResponse](f, err)
}

func (a adapter) DeleteFile(ctx context.Context, owner, repo, path string, in apprepo.FileDeleteInput) error {
	return a.p.DeleteFile(ctx, owner, repo, path, forgejo_sdk.DeleteFileOptions{
		FileOptions: fileOptions(in.Message, in.Branch, in.NewBranch),
		SHA:         in.SHA,
	})
}

func (a adapter) ChangeFiles(ctx context.Context, owner, repo string, in apprepo.ChangeFilesInput) (map[string]any, error) {
	files := make([]forge.ChangeFileOperation, 0, len(in.Files))
	for _, f := range in.Files {
		files = append(files, forge.ChangeFileOperation{Operation: f.Operation, Path: f.Path, Content: f.Content, SHA: f.SHA})
	}
	return a.p.ChangeFiles(ctx, owner, repo, forge.ChangeFilesOptions{
		BranchName:    in.Branch,
		NewBranchName: in.NewBranch,
		Message:       in.Message,
		Files:         files,
	})
}
