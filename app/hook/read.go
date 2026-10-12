// SPDX-License-Identifier: GPL-3.0-or-later

package hook

import (
	"context"
	"fmt"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
)

// Collection is the leading part of a repository's webhooks, each in its safe
// form. Truncated is true when the repository has more webhooks than were asked
// for.
type Collection struct {
	Hooks     []model.HookPayload
	Truncated bool
}

// ReadCollection returns up to limit of a repository's webhooks in their safe
// form, in the order Forgejo lists them, and whether more exist. It is the read
// behind the hooks collection resource; list_repo_hooks pages through the rest.
// limit must be positive.
func (s *Service) ReadCollection(ctx context.Context, owner, repo string, limit int) (Collection, error) {
	// One webhook more than the limit, so that "more than the limit" is visible
	// without a second call.
	list, err := s.port.ListRepoHooks(ctx, owner, repo, Paging{Page: 1, Limit: limit + 1})
	if err != nil {
		return Collection{}, fmt.Errorf("list repo hooks: %w", err)
	}
	truncated := len(list.Hooks) > limit
	shown := list.Hooks
	if truncated {
		shown = shown[:limit]
	}
	payloads := make([]model.HookPayload, len(shown))
	for i, hk := range shown {
		payloads[i] = model.SafeHook(hk)
	}
	return Collection{Hooks: payloads, Truncated: truncated}, nil
}
