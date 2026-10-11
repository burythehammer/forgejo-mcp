// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"errors"
	"net/http"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"

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

func (a *Adapter) DeleteFile(_ context.Context, owner, repo, path string, opt forgejo_sdk.DeleteFileOptions) error {
	resp, err := a.client.DeleteFile(owner, repo, path, opt)
	return statusErr(resp, err)
}

// ChangeFiles has no SDK method, so it goes over raw HTTP to the contents
// endpoint with the package-level target and credential policy.
func (a *Adapter) ChangeFiles(ctx context.Context, owner, repo string, opt forge.ChangeFilesOptions) (map[string]any, error) {
	var out map[string]any
	path := forgejo.APIPath("repos", owner, repo, "contents")
	if err := forgejo.DoJSON(ctx, http.MethodPost, path, opt, &out); err != nil {
		return nil, rawStatusErr(err)
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
		out.TotalCount = forgejo.TotalCountPtr(resp.Header)
	}
	return out, nil
}

// rawStatusErr carries the status of a raw-HTTP failure the way statusErr does
// for the SDK's, leaving the message unchanged.
func rawStatusErr(err error) error {
	var he *forgejo.HTTPError
	if errors.As(err, &he) {
		return &forge.StatusError{StatusCode: he.StatusCode, Err: err}
	}
	return err
}
