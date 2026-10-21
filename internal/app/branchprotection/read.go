// SPDX-License-Identifier: GPL-3.0-or-later

package branchprotection

import (
	"context"
	"fmt"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
)

// Collection is the leading part of a repository's branch protection rules.
// Truncated is true when the repository has more rules than were asked for.
type Collection struct {
	Rules     []*model.BranchProtection
	Truncated bool
}

// ReadCollection returns up to limit of a repository's rules, in the order
// Forgejo lists them, and whether more exist. It is the read behind the branch
// protections collection resource; list_branch_protections pages through the
// rest. limit must be positive.
func (s *Service) ReadCollection(ctx context.Context, owner, repo string, limit int) (Collection, error) {
	// One rule more than the limit, so that "more than the limit" is visible
	// without a second call.
	rules, err := s.port.ListBranchProtections(ctx, owner, repo, Paging{Page: 1, Limit: limit + 1})
	if err != nil {
		return Collection{}, fmt.Errorf("list branch protections err: %w", err)
	}
	truncated := len(rules) > limit
	if truncated {
		rules = rules[:limit]
	}
	return Collection{Rules: rules, Truncated: truncated}, nil
}
