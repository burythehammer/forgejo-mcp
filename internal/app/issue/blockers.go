// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"errors"
	"fmt"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/list"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
)

// Blocked is one listed issue with the issues it depends on. Repo is the full
// name of the repository the issue was listed from, so a blocker can be named
// relative to it.
type Blocked struct {
	Issue     *model.Issue
	Repo      string
	BlockedBy []*model.Issue
}

// ListAllBlocked is ListAll with each issue's blockers. It makes one dependency
// read per listed issue, the fan-out ADR 0007 allows for this list because
// Forgejo has no endpoint that returns blockers in bulk.
func (s *Service) ListAllBlocked(ctx context.Context, in ListInput) ([]Blocked, error) {
	issues, err := s.ListAll(ctx, in)
	if err != nil {
		return nil, err
	}
	return s.withBlockers(ctx, issues, func(*model.Issue) (string, string, error) {
		return in.Owner, in.Repo, nil
	})
}

// SearchAllBlocked is SearchAll with each issue's blockers. Each issue is read
// in its own repository, which search_issues reports on the issue.
func (s *Service) SearchAllBlocked(ctx context.Context, in SearchInput) ([]Blocked, error) {
	issues, err := s.SearchAll(ctx, in)
	if err != nil {
		return nil, err
	}
	return s.withBlockers(ctx, issues, repositoryOf)
}

// repositoryOf names the repository a searched issue belongs to. A search
// result that carries no repository cannot have its blockers read, so it fails.
func repositoryOf(iss *model.Issue) (string, string, error) {
	if iss.Repository == nil {
		return "", "", fmt.Errorf("issue #%d has no repository in its record; nothing was written", iss.Index)
	}
	return iss.Repository.Owner, iss.Repository.Name, nil
}

// withBlockers reads the blockers of each issue, in list order. Each issue's
// dependency list is read page by page through the list module under the
// file-output ceiling, so a long list of blockers is read whole or refused, never
// truncated. Any failed read fails the whole call; no partial result is returned.
func (s *Service) withBlockers(ctx context.Context, issues []*model.Issue,
	repoOf func(*model.Issue) (string, string, error),
) ([]Blocked, error) {
	ceiling, known := s.port.MaxResponseItems(ctx)
	limit := list.DefaultLimit(defaultLimit, list.Ceiling{Max: ceiling, Known: known})
	out := make([]Blocked, 0, len(issues))
	for _, iss := range issues {
		owner, repo, err := repoOf(iss)
		if err != nil {
			return nil, err
		}
		blockers, err := list.ReadWhole(ctx, limit, list.FileCeiling,
			func(ctx context.Context, p list.Paging) (list.Batch[*model.Issue], error) {
				got, err := s.port.ListIssueDependencies(ctx, owner, repo, iss.Index, Paging(p))
				if err != nil {
					return list.Batch[*model.Issue]{}, fmt.Errorf("read the blockers of %s/%s#%d err: %w", owner, repo, iss.Index, err)
				}
				return list.Batch[*model.Issue]{Items: got}, nil
			})
		var over *list.OverCeiling
		if errors.As(err, &over) {
			return nil, fmt.Errorf("issue %s/%s#%d has more than %d blockers; nothing was written", owner, repo, iss.Index, over.Max)
		}
		if err != nil {
			return nil, err
		}
		out = append(out, Blocked{Issue: iss, Repo: owner + "/" + repo, BlockedBy: blockers})
	}
	return out, nil
}
