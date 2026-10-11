// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"net/http"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var _ forge.Hooks = (*Adapter)(nil)

func (a *Adapter) ListRepoHooks(_ context.Context, owner, repo string, opt forgejo_sdk.ListHooksOptions) (forge.HookList, error) {
	hooks, resp, err := a.client.ListRepoHooks(owner, repo, opt)
	if err != nil {
		return forge.HookList{}, statusErr(resp, err)
	}
	out := forge.HookList{Hooks: hooks}
	if resp != nil && resp.Response != nil {
		out.TotalCount = totalCountPtr(resp.Header)
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
	return a.rawDo(ctx, http.MethodPost, forge.APIPath("repos", owner, repo, "hooks", id, "tests"), nil, nil)
}
