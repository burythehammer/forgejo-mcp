// SPDX-License-Identifier: GPL-3.0-or-later

package branchprotection

import (
	"context"
	"fmt"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/list"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/model"
)

// ReadCollection returns one page of a repository's rules, in the order
// Forgejo lists them, with whether more follow. It is the read behind the branch
// protections collection resource at the page and limit the caller names;
// p.Limit must be positive. The listing reports no repository total, so the
// result's Total is unknown.
func (s *Service) ReadCollection(ctx context.Context, owner, repo string, p list.Paging) (list.Bounded[*model.BranchProtection], error) {
	return list.Run(ctx, list.Request{Page: p.Page, Limit: p.Limit}, list.Ceiling{},
		func(ctx context.Context, q list.Paging) (list.Batch[*model.BranchProtection], error) {
			rules, err := s.port.ListBranchProtections(ctx, owner, repo, Paging{Page: q.Page, Limit: q.Limit})
			if err != nil {
				return list.Batch[*model.BranchProtection]{}, fmt.Errorf("list branch protections err: %w", err)
			}
			return list.Batch[*model.BranchProtection]{Items: rules}, nil
		})
}
