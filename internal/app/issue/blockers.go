// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"fmt"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/list"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/model"
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
	return s.attachBlockers(ctx, issues, func(iss *model.Issue) (Target, error) {
		return Target{Owner: in.Owner, Repo: in.Repo, Index: iss.Index}, nil
	})
}

// SearchAllBlocked is SearchAll with each issue's blockers. Each issue is read
// in its own repository, which search_issues reports on the issue.
func (s *Service) SearchAllBlocked(ctx context.Context, in SearchInput) ([]Blocked, error) {
	issues, err := s.SearchAll(ctx, in)
	if err != nil {
		return nil, err
	}
	return s.attachBlockers(ctx, issues, targetOf)
}

// targetOf names a searched issue by the repository it belongs to. A search
// result that carries no repository cannot have its blockers read, so it fails.
func targetOf(iss *model.Issue) (Target, error) {
	if iss.Repository == nil {
		return Target{}, fmt.Errorf("issue #%d has no repository in its record; nothing was written", iss.Index)
	}
	return Target{Owner: iss.Repository.Owner, Repo: iss.Repository.Name, Index: iss.Index}, nil
}

// attachBlockers reads the blockers of each issue, in list order, through
// readWhole, the same path a listing takes: the file-output ceiling applies to
// each issue's blockers, and an over-ceiling or failed read fails the whole
// call with no partial result.
func (s *Service) attachBlockers(ctx context.Context, issues []*model.Issue,
	at func(*model.Issue) (Target, error),
) ([]Blocked, error) {
	out := make([]Blocked, 0, len(issues))
	for _, iss := range issues {
		t, err := at(iss)
		if err != nil {
			return nil, err
		}
		noun := fmt.Sprintf("blockers of %s/%s#%d", t.Owner, t.Repo, t.Index)
		blockers, err := s.readWhole(ctx, s.fetchBlockers(t), noun)
		if err != nil {
			return nil, err
		}
		out = append(out, Blocked{Issue: iss, Repo: t.Owner + "/" + t.Repo, BlockedBy: blockers})
	}
	return out, nil
}

// fetchBlockers returns the list.Fetch that reads one page of the dependencies
// of the issue t from the port.
func (s *Service) fetchBlockers(t Target) list.Fetch[*model.Issue] {
	return func(ctx context.Context, p list.Paging) (list.Batch[*model.Issue], error) {
		got, err := s.port.ListIssueDependencies(ctx, t.Owner, t.Repo, t.Index, Paging(p))
		if err != nil {
			return list.Batch[*model.Issue]{}, fmt.Errorf("read the blockers of %s/%s#%d err: %w", t.Owner, t.Repo, t.Index, err)
		}
		return list.Batch[*model.Issue]{Items: got}, nil
	}
}
