// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var _ forge.Hooks = (*Adapter)(nil)

// WithRawBaseURL returns a copy of the adapter whose raw-HTTP methods (the ones
// the SDK has no call for) talk to the Forgejo at url rather than the globally
// configured one. The factory sets it from Config; contract tests point it at
// the stub. Their credential is still the request's, via ctx.
func (a *Adapter) WithRawBaseURL(url string) *Adapter {
	c := *a
	c.rawBase = url
	return &c
}

func (a *Adapter) ListRepoHooks(_ context.Context, owner, repo string, opt forgejo_sdk.ListHooksOptions) (forge.HookList, error) {
	hooks, resp, err := a.client.ListRepoHooks(owner, repo, opt)
	if err != nil {
		return forge.HookList{}, statusErr(resp, err)
	}
	out := forge.HookList{Hooks: hooks}
	if resp != nil && resp.Response != nil {
		out.TotalCount = forgejo.TotalCountPtr(resp.Header)
	}
	return out, nil
}

func (a *Adapter) GetRepoHook(_ context.Context, owner, repo string, id int64) (*forgejo_sdk.Hook, error) {
	h, resp, err := a.client.GetRepoHook(owner, repo, id)
	return h, statusErr(resp, err)
}

func (a *Adapter) CreateRepoHook(_ context.Context, owner, repo string, opt forgejo_sdk.CreateHookOption) (*forgejo_sdk.Hook, error) {
	h, resp, err := a.client.CreateRepoHook(owner, repo, opt)
	return h, statusErr(resp, err)
}

func (a *Adapter) EditRepoHook(_ context.Context, owner, repo string, id int64, opt forgejo_sdk.EditHookOption) error {
	resp, err := a.client.EditRepoHook(owner, repo, id, opt)
	return statusErr(resp, err)
}

func (a *Adapter) DeleteRepoHook(_ context.Context, owner, repo string, id int64) error {
	resp, err := a.client.DeleteRepoHook(owner, repo, id)
	return statusErr(resp, err)
}

// TestRepoHook is not in SDK v3: POST /repos/{owner}/{repo}/hooks/{id}/tests.
func (a *Adapter) TestRepoHook(ctx context.Context, owner, repo string, id int64) error {
	path := forgejo.APIPath("repos", owner, repo, "hooks", id, "tests")
	if a.rawBase != "" {
		path = strings.TrimRight(a.rawBase, "/") + "/api/v1" + path
	}
	err := forgejo.DoJSON(ctx, http.MethodPost, path, nil, nil)
	var he *forgejo.HTTPError
	if errors.As(err, &he) {
		return &forge.StatusError{StatusCode: he.StatusCode, Err: err}
	}
	return err
}
