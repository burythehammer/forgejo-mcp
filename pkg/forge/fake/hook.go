// SPDX-License-Identifier: GPL-3.0-or-later

package fake

import (
	"context"
	"sync"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var _ forge.Hooks = (*Forgejo)(nil)

type hookState struct {
	mu    sync.Mutex
	hooks map[string][]*forgejo_sdk.Hook // key "owner/repo", creation order
	next  int64
	// authHeaders holds the write-only authorization header by hook id. The
	// port's Hook has no field for it, so it is kept here, not in Config.
	authHeaders map[int64]string
}

func (f *Forgejo) hookList(owner, repo string) ([]*forgejo_sdk.Hook, error) {
	if !f.settingsRepoExists(owner, repo) {
		return nil, notFound("repository")
	}
	return f.hooks.hooks[owner+"/"+repo], nil
}

func findHook(hooks []*forgejo_sdk.Hook, id int64) (int, error) {
	for i, h := range hooks {
		if h.ID == id {
			return i, nil
		}
	}
	return -1, notFound("hook")
}

func copyHook(h *forgejo_sdk.Hook) *forgejo_sdk.Hook {
	c := *h
	c.Config = make(map[string]string, len(h.Config))
	for k, v := range h.Config {
		c.Config[k] = v
	}
	c.Events = append([]string(nil), h.Events...)
	return &c
}

func (f *Forgejo) ListRepoHooks(_ context.Context, owner, repo string, opt forgejo_sdk.ListHooksOptions) (forge.HookList, error) {
	f.hooks.mu.Lock()
	defer f.hooks.mu.Unlock()
	hooks, err := f.hookList(owner, repo)
	if err != nil {
		return forge.HookList{}, err
	}
	lo, hi := page(opt.ListOptions, len(hooks))
	out := make([]*forgejo_sdk.Hook, 0, hi-lo)
	for _, h := range hooks[lo:hi] {
		out = append(out, copyHook(h))
	}
	total := len(hooks)
	return forge.HookList{Hooks: out, TotalCount: &total}, nil
}

func (f *Forgejo) GetRepoHook(_ context.Context, owner, repo string, id int64) (*forgejo_sdk.Hook, error) {
	f.hooks.mu.Lock()
	defer f.hooks.mu.Unlock()
	hooks, err := f.hookList(owner, repo)
	if err != nil {
		return nil, err
	}
	i, err := findHook(hooks, id)
	if err != nil {
		return nil, err
	}
	return copyHook(hooks[i]), nil
}

func (f *Forgejo) CreateRepoHook(_ context.Context, owner, repo string, opt forgejo_sdk.CreateHookOption) (*forgejo_sdk.Hook, error) {
	if err := opt.Validate(); err != nil {
		return nil, err
	}
	f.hooks.mu.Lock()
	defer f.hooks.mu.Unlock()
	hooks, err := f.hookList(owner, repo)
	if err != nil {
		return nil, err
	}
	f.hooks.next++
	now := f.Now()
	h := &forgejo_sdk.Hook{
		ID:      f.hooks.next,
		Type:    string(opt.Type),
		Config:  map[string]string{},
		Events:  opt.Events,
		Active:  opt.Active,
		Created: now,
		Updated: now,
	}
	for k, v := range opt.Config {
		h.Config[k] = v
	}
	if opt.BranchFilter != "" {
		h.Config["branch_filter"] = opt.BranchFilter
	}
	if f.hooks.hooks == nil {
		f.hooks.hooks = map[string][]*forgejo_sdk.Hook{}
	}
	f.hooks.hooks[owner+"/"+repo] = append(hooks, h)
	return copyHook(h), nil
}

// EditRepoHook follows Forgejo's partial-update rule: a field the edit leaves
// nil keeps its stored value; a field it sets is replaced, and an explicit
// empty branch filter clears it.
func (f *Forgejo) EditRepoHook(_ context.Context, owner, repo string, id int64, opt forge.HookEdit) error {
	f.hooks.mu.Lock()
	defer f.hooks.mu.Unlock()
	hooks, err := f.hookList(owner, repo)
	if err != nil {
		return err
	}
	i, err := findHook(hooks, id)
	if err != nil {
		return err
	}
	h := hooks[i]
	if opt.Config != nil {
		h.Config = map[string]string{}
		for k, v := range opt.Config {
			h.Config[k] = v
		}
	}
	if opt.Events != nil {
		h.Events = opt.Events
	}
	if opt.BranchFilter != nil {
		if *opt.BranchFilter == "" {
			delete(h.Config, "branch_filter")
		} else {
			h.Config["branch_filter"] = *opt.BranchFilter
		}
	}
	if opt.Active != nil {
		h.Active = *opt.Active
	}
	if opt.AuthorizationHeader != nil {
		if f.hooks.authHeaders == nil {
			f.hooks.authHeaders = map[int64]string{}
		}
		f.hooks.authHeaders[id] = *opt.AuthorizationHeader
	}
	h.Updated = f.Now()
	return nil
}

func (f *Forgejo) DeleteRepoHook(_ context.Context, owner, repo string, id int64) error {
	f.hooks.mu.Lock()
	defer f.hooks.mu.Unlock()
	hooks, err := f.hookList(owner, repo)
	if err != nil {
		return err
	}
	i, err := findHook(hooks, id)
	if err != nil {
		return err
	}
	f.hooks.hooks[owner+"/"+repo] = append(hooks[:i:i], hooks[i+1:]...)
	return nil
}

func (f *Forgejo) TestRepoHook(_ context.Context, owner, repo string, id int64) error {
	f.hooks.mu.Lock()
	defer f.hooks.mu.Unlock()
	hooks, err := f.hookList(owner, repo)
	if err != nil {
		return err
	}
	if _, err := findHook(hooks, id); err != nil {
		return err
	}
	return nil
}

// HookAuthorizationHeader reports the authorization header last set on a hook
// by an edit, which the port cannot observe: the port's Hook has no field for
// it, so the header is write-only through the port.
func (f *Forgejo) HookAuthorizationHeader(id int64) string {
	f.hooks.mu.Lock()
	defer f.hooks.mu.Unlock()
	return f.hooks.authHeaders[id]
}
