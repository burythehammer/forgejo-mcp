// SPDX-License-Identifier: GPL-3.0-or-later

// Package hookadapter implements the webhook use cases' port (internal/app/hook.Port)
// over the Forgejo port (forge.Hooks). It is the one place that translates
// between repo-owned types and SDK types for the webhook domain: inputs become
// SDK options, SDK results become internal/app/model values through their JSON form. A
// Forgejo status failure on an edit is tagged as the use case's StatusError,
// so the use case can report the status without importing the forge package.
package hookadapter

import (
	"context"

	apphook "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/hook"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/model"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge/bridge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// New returns the use cases' port over p.
func New(p Source) apphook.Port { return adapter{p: p} }

// Source is the part of the Forgejo port the webhook domain uses: its hook
// methods and the instance's pagination ceiling.
type Source interface {
	forge.Hooks
	forge.Ceilings
}

type adapter struct{ p Source }

// MaxResponseItems reads the instance's pagination ceiling through the source.
func (a adapter) MaxResponseItems(ctx context.Context) (int, bool) {
	return a.p.MaxResponseItems(ctx)
}

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
