// SPDX-License-Identifier: GPL-3.0-or-later

package hook

import (
	"context"
	"fmt"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/list"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/model"
)

// ReadCollection returns one page of a repository's webhooks in their safe form,
// in the order Forgejo lists them, with whether more follow. It is the read
// behind the hooks collection resource at the page and limit the caller names;
// p.Limit must be positive. The grand total is the server's when it reports one.
func (s *Service) ReadCollection(ctx context.Context, owner, repo string, p list.Paging) (list.Bounded[model.HookPayload], error) {
	return list.Run(ctx, list.Request{Page: p.Page, Limit: p.Limit}, list.Ceiling{},
		func(ctx context.Context, q list.Paging) (list.Batch[model.HookPayload], error) {
			hooks, err := s.port.ListRepoHooks(ctx, owner, repo, Paging{Page: q.Page, Limit: q.Limit})
			if err != nil {
				return list.Batch[model.HookPayload]{}, fmt.Errorf("list repo hooks: %w", err)
			}
			return list.Batch[model.HookPayload]{Items: safeHooks(hooks.Hooks), Total: hooks.TotalCount}, nil
		})
}

// safeHooks is each webhook in its safe form, the only form a use case returns.
func safeHooks(hooks []*model.Hook) []model.HookPayload {
	out := make([]model.HookPayload, len(hooks))
	for i, hk := range hooks {
		out[i] = model.SafeHook(hk)
	}
	return out
}
