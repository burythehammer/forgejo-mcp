// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var _ forge.RepoFiles = (*Adapter)(nil)

// contentsBodyCap bounds the JSON body of one file's contents response. Its
// Base64 content is 4/3 of the file, and the rest is a fixed envelope.
const contentsBodyCap = forge.MaxFileSourceBytes/3*4 + 64<<10

// GetFile reads a file's raw bytes. A file over forge.MaxFileSourceBytes is
// refused with forge.ErrPayloadTooLarge rather than read whole.
func (a *Adapter) GetFile(_ context.Context, owner, repo, ref, path string) ([]byte, error) {
	rc, resp, err := a.client.GetFileReader(owner, repo, ref, path)
	if err != nil || rc == nil {
		// On a server failure the SDK still returns a reader over the error body.
		if rc != nil {
			_ = rc.Close()
		}
		return nil, statusErr(resp, err)
	}
	defer rc.Close()
	b, err := io.ReadAll(io.LimitReader(rc, forge.MaxFileSourceBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read file body: %w", err)
	}
	if int64(len(b)) > forge.MaxFileSourceBytes {
		return nil, forge.ErrPayloadTooLarge
	}
	return b, nil
}

// GetContents returns a file's metadata and Base64 content. The body is read
// through a size cap, and its path is built with forge.APIPath segment by
// segment, so the slashes of a nested file path are kept.
func (a *Adapter) GetContents(ctx context.Context, owner, repo, ref, path string) (*forgejo_sdk.ContentsResponse, error) {
	segs := []any{"repos", owner, repo, "contents"}
	for _, s := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		segs = append(segs, s)
	}
	apiPath, err := forge.APIPath(segs...)
	if err != nil {
		return nil, err
	}
	resp, err := a.raw.doAPIRaw(ctx, apiPath+"?ref="+url.QueryEscape(ref), "application/json", "", contentsBodyCap)
	if err != nil {
		return nil, err
	}
	var c forgejo_sdk.ContentsResponse
	if err := json.Unmarshal(resp.Body, &c); err != nil {
		return nil, fmt.Errorf("expect file, got directory")
	}
	return &c, nil
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
	p, err := contentsPath(owner, repo, path)
	if err != nil {
		return err
	}
	status, body, err := a.raw.doStatus(ctx, http.MethodDelete, p, opt)
	if err != nil {
		return err
	}
	return answered(status, body, http.StatusOK, http.StatusNoContent)
}

// ChangeFiles has no SDK method, so it goes over raw HTTP to the contents
// endpoint, through the adapter's raw client.
func (a *Adapter) ChangeFiles(ctx context.Context, owner, repo string, opt forge.ChangeFilesOptions) (map[string]any, error) {
	var out map[string]any
	path, err := forge.APIPath("repos", owner, repo, "contents")
	if err != nil {
		return nil, err
	}
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
