// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"net/http"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var _ forge.Hooks = (*Adapter)(nil)

// hookWire is a hook as Forgejo serves it. Forgejo puts branch_filter at the
// top level, not inside config, and the SDK's Hook type has no field for it.
// Embedding the SDK type keeps its other fields as they are on the wire.
type hookWire struct {
	forgejo_sdk.Hook
	BranchFilter string `json:"branch_filter"`
}

// getRepoHook reads one hook over raw HTTP, so that the top-level branch_filter
// is kept. A non-empty filter is exposed in Config["branch_filter"], where the
// fake keeps it too.
func (a *Adapter) getRepoHook(ctx context.Context, owner, repo string, id int64) (*forgejo_sdk.Hook, error) {
	var w hookWire
	if err := a.rawDo(ctx, http.MethodGet, forge.APIPath("repos", owner, repo, "hooks", id), nil, &w); err != nil {
		return nil, err
	}
	h := w.Hook
	if w.BranchFilter != "" {
		if h.Config == nil {
			h.Config = map[string]string{}
		}
		h.Config["branch_filter"] = w.BranchFilter
	}
	return &h, nil
}

func (a *Adapter) ListRepoHooks(ctx context.Context, owner, repo string, opt forgejo_sdk.ListHooksOptions) (forge.HookList, error) {
	hooks, resp, err := a.client.ListRepoHooks(owner, repo, opt)
	if err != nil {
		return forge.HookList{}, statusErr(resp, err)
	}
	out := forge.HookList{Hooks: hooks}
	if resp != nil && resp.Response != nil {
		out.TotalCount = totalCountPtr(resp.Header)
	}
	// The SDK's list drops the top-level branch_filter, so each hook on the
	// page is read again to fill it in.
	for i, h := range out.Hooks {
		full, err := a.getRepoHook(ctx, owner, repo, h.ID)
		if err != nil {
			return forge.HookList{}, err
		}
		out.Hooks[i] = full
	}
	return out, nil
}

func (a *Adapter) GetRepoHook(ctx context.Context, owner, repo string, id int64) (*forgejo_sdk.Hook, error) {
	return a.getRepoHook(ctx, owner, repo, id)
}

func (a *Adapter) CreateRepoHook(_ context.Context, owner, repo string, opt forgejo_sdk.CreateHookOption) (*forgejo_sdk.Hook, error) {
	h, resp, err := a.client.CreateRepoHook(owner, repo, opt)
	return h, statusErr(resp, err)
}

// EditRepoHook sends a raw PATCH. Forgejo clears a branch filter that a PATCH
// omits, and the SDK's EditHookOption always marshals every field, so the
// stored hook is read first. An unset branch filter, events or active is
// resent with its stored value, and the write-only authorization header and
// config go out only when the caller set them. An explicit empty branch
// filter still clears it.
func (a *Adapter) EditRepoHook(ctx context.Context, owner, repo string, id int64, opt forge.HookEdit) error {
	stored, err := a.getRepoHook(ctx, owner, repo, id)
	if err != nil {
		return err
	}
	body := map[string]any{}
	if opt.Config != nil {
		body["config"] = opt.Config
	}
	events := stored.Events
	if opt.Events != nil {
		events = opt.Events
	}
	if events == nil {
		events = []string{}
	}
	body["events"] = events
	filter := stored.Config["branch_filter"]
	if opt.BranchFilter != nil {
		filter = *opt.BranchFilter
	}
	body["branch_filter"] = filter
	active := stored.Active
	if opt.Active != nil {
		active = *opt.Active
	}
	body["active"] = active
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
