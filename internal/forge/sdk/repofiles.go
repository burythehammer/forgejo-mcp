// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"net/http"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var _ forge.RepoFiles = (*Adapter)(nil)

func (a *Adapter) GetFile(_ context.Context, owner, repo, ref, path string) ([]byte, error) {
	b, resp, err := a.client.GetFile(owner, repo, ref, path)
	return b, statusErr(resp, err)
}

func (a *Adapter) GetContents(_ context.Context, owner, repo, ref, path string) (*forgejo_sdk.ContentsResponse, error) {
	c, resp, err := a.client.GetContents(owner, repo, ref, path)
	return c, statusErr(resp, err)
}

func (a *Adapter) ListContents(_ context.Context, owner, repo, ref, path string) ([]*forgejo_sdk.ContentsResponse, error) {
	c, resp, err := a.client.ListContents(owner, repo, ref, path)
	return c, statusErr(resp, err)
}

func (a *Adapter) GetTree(_ context.Context, owner, repo, ref string, opt forgejo_sdk.GetTreesOptions) (*forgejo_sdk.GitTreeResponse, error) {
	t, resp, err := a.client.GetTrees(owner, repo, ref, opt)
	return t, statusErr(resp, err)
}

func (a *Adapter) CreateFile(_ context.Context, owner, repo, path string, opt forgejo_sdk.CreateFileOptions) (*forgejo_sdk.FileResponse, error) {
	r, resp, err := a.client.CreateFile(owner, repo, path, opt)
	return r, statusErr(resp, err)
}

func (a *Adapter) UpdateFile(_ context.Context, owner, repo, path string, opt forgejo_sdk.UpdateFileOptions) (*forgejo_sdk.FileResponse, error) {
	r, resp, err := a.client.UpdateFile(owner, repo, path, opt)
	return r, statusErr(resp, err)
}

// DeleteFile is raw for the reason given at SetRepoActionSecret: the SDK drops
// the body of a refusal. Forgejo defaults an empty branch to the default branch,
// so the SDK's branch lookup for old servers is not repeated here.
func (a *Adapter) DeleteFile(ctx context.Context, owner, repo, path string, opt forgejo_sdk.DeleteFileOptions) error {
	status, body, err := a.raw.doStatus(ctx, http.MethodDelete, contentsPath(owner, repo, path), opt)
	if err != nil {
		return err
	}
	return answered(status, body, http.StatusOK, http.StatusNoContent)
}

// ChangeFiles has no SDK method, so it goes over raw HTTP to the contents
// endpoint, through the adapter's raw client.
func (a *Adapter) ChangeFiles(ctx context.Context, owner, repo string, opt forge.ChangeFilesOptions) (map[string]any, error) {
	var out map[string]any
	path := forge.APIPath("repos", owner, repo, "contents")
	if err := a.rawDo(ctx, http.MethodPost, path, opt, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (a *Adapter) GetCommit(_ context.Context, owner, repo, sha string) (*forgejo_sdk.Commit, error) {
	c, resp, err := a.client.GetSingleCommit(owner, repo, sha)
	return c, statusErr(resp, err)
}

func (a *Adapter) ListCommits(_ context.Context, owner, repo string, opt forgejo_sdk.ListCommitOptions) ([]*forgejo_sdk.Commit, error) {
	c, resp, err := a.client.ListRepoCommits(owner, repo, opt)
	return c, statusErr(resp, err)
}

func (a *Adapter) ListCommitStatuses(_ context.Context, owner, repo, sha string, opt forgejo_sdk.ListStatusesOption) (forge.CommitStatuses, error) {
	s, resp, err := a.client.ListStatuses(owner, repo, sha, opt)
	if err != nil {
		return forge.CommitStatuses{}, statusErr(resp, err)
	}
	out := forge.CommitStatuses{Statuses: s}
	if resp != nil && resp.Response != nil {
		out.TotalCount = totalCountPtr(resp.Header)
	}
	return out, nil
}
