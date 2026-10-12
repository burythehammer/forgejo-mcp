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

// EditRepoHook sends a raw PATCH carrying only the fields the caller set. The
// SDK's EditHookOption always marshals every field, which made an edit of one
// field send empty values for the rest.
func (a *Adapter) EditRepoHook(ctx context.Context, owner, repo string, id int64, opt forge.HookEdit) error {
	body := map[string]any{}
	if opt.Config != nil {
		body["config"] = opt.Config
	}
	if opt.Events != nil {
		body["events"] = opt.Events
	}
	if opt.BranchFilter != nil {
		body["branch_filter"] = *opt.BranchFilter
	}
	if opt.Active != nil {
		body["active"] = *opt.Active
	}
	if opt.AuthorizationHeader != nil {
		body["authorization_header"] = *opt.AuthorizationHeader
	}
	return a.rawDo(ctx, http.MethodPatch, forge.APIPath("repos", owner, repo, "hooks", id), body, nil)
}

func (a *Adapter) DeleteRepoHook(_ context.Context, owner, repo string, id int64) error {
	resp, err := a.client.DeleteRepoHook(owner, repo, id)
	return statusErr(resp, err)
}

// TestRepoHook is not in SDK v3: POST /repos/{owner}/{repo}/hooks/{id}/tests.
func (a *Adapter) TestRepoHook(ctx context.Context, owner, repo string, id int64) error {
	return a.rawDo(ctx, http.MethodPost, forge.APIPath("repos", owner, repo, "hooks", id, "tests"), nil, nil)
}
