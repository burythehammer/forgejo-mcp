// SPDX-License-Identifier: GPL-3.0-or-later

// Package hookadapter implements the webhook use cases' port (app/hook.Port)
// over the Forgejo port (forge.Hooks). It is the one place that translates
// between repo-owned types and SDK types for the webhook domain: inputs become
// SDK options, SDK results become app/model values through their JSON form. A
// Forgejo status failure on an edit is tagged as the use case's StatusError,
// so the use case can report the status without importing the forge package.
package hookadapter

import (
	"context"

	apphook "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/hook"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/bridge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// New returns the use cases' port over p.
func New(p forge.Hooks) apphook.Port { return adapter{p: p} }

type adapter struct{ p forge.Hooks }

var _ apphook.Port = adapter{}

func (a adapter) ListRepoHooks(ctx context.Context, owner, repo string, p apphook.Paging) (model.HookList, error) {
	list, err := a.p.ListRepoHooks(ctx, owner, repo, forgejo_sdk.ListHooksOptions{
		ListOptions: forgejo_sdk.ListOptions{Page: p.Page, PageSize: p.Limit},
	})
	if err != nil {
		return model.HookList{}, err
	}
	hooks, err := bridge.Convert[[]*model.Hook](list.Hooks, nil)
	return model.HookList{Hooks: hooks, TotalCount: list.TotalCount}, err
}

func (a adapter) GetRepoHook(ctx context.Context, owner, repo string, id int64) (*model.Hook, error) {
	hk, err := a.p.GetRepoHook(ctx, owner, repo, id)
	return bridge.Convert[*model.Hook](hk, err)
}

func (a adapter) CreateRepoHook(ctx context.Context, owner, repo string, in apphook.CreateInput) (*model.Hook, error) {
	hk, err := a.p.CreateRepoHook(ctx, owner, repo, forgejo_sdk.CreateHookOption{
		Type:         forgejo_sdk.HookType(in.Type),
		Config:       in.Config,
		Events:       in.Events,
		BranchFilter: in.BranchFilter,
		Active:       in.Active,
	})
	return bridge.Convert[*model.Hook](hk, err)
}

func (a adapter) EditRepoHook(ctx context.Context, owner, repo string, id int64, in apphook.EditInput) error {
	err := a.p.EditRepoHook(ctx, owner, repo, id, forge.HookEdit{
		Config:       in.Config,
		Events:       in.Events,
		BranchFilter: in.BranchFilter,
		Active:       in.Active,
	})
	if code := forge.StatusCode(err); code != 0 {
		return &apphook.StatusError{Code: code, Err: err}
	}
	return err
}

func (a adapter) DeleteRepoHook(ctx context.Context, owner, repo string, id int64) error {
	return a.p.DeleteRepoHook(ctx, owner, repo, id)
}

func (a adapter) TestRepoHook(ctx context.Context, owner, repo string, id int64) error {
	return a.p.TestRepoHook(ctx, owner, repo, id)
}
