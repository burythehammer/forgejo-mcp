// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"fmt"
	"strings"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/list"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/model"
)

// Dependencies lists the issues that an issue depends on, one page at a time.
func (s *Service) Dependencies(ctx context.Context, t Target, p Paging) (model.DependencyPage, error) {
	return s.relatedPage(ctx, "dependencies", s.port.ListIssueDependencies, t, p)
}

// Dependents lists the issues that depend on an issue, one page at a time.
func (s *Service) Dependents(ctx context.Context, t Target, p Paging) (model.DependencyPage, error) {
	return s.relatedPage(ctx, "dependents", s.port.ListIssueDependents, t, p)
}

// relatedPage reads one page of a dependency list through the list module, so
// a limit above the instance ceiling is refused and a default is lowered to it.
func (s *Service) relatedPage(ctx context.Context, what string,
	fetch func(ctx context.Context, owner, repo string, index int64, p Paging) ([]*model.Issue, error),
	t Target, p Paging,
) (model.DependencyPage, error) {
	page, err := runPage(ctx, s.port, list.Paging(p), defaultLimit,
		func(ctx context.Context, q list.Paging) ([]*model.Issue, error) {
			got, err := fetch(ctx, t.Owner, t.Repo, t.Index, Paging(q))
			if err != nil {
				return nil, fmt.Errorf("list issue %s err: %w", what, err)
			}
			return got, nil
		})
	if err != nil {
		return model.DependencyPage{}, err
	}
	return model.DependencyPage{
		Issues: page.Items, PageMeta: model.PageMetaOf(page),
	}, nil
}

// AddDependency makes the issue t depend on dep, which may be in another
// repository. An issue cannot depend on itself. Repository names compare
// without case, as Forgejo's do, so that the refusal is clear rather than a
// server error; a same-index issue in a different repository is a legitimate
// cross-repository dependency.
func (s *Service) AddDependency(ctx context.Context, t Target, dep Target) error {
	if strings.EqualFold(dep.Owner, t.Owner) && strings.EqualFold(dep.Repo, t.Repo) && dep.Index == t.Index {
		return fmt.Errorf("an issue cannot depend on itself")
	}
	if err := s.port.AddIssueDependency(ctx, t.Owner, t.Repo, t.Index, dep); err != nil {
		return fmt.Errorf("add issue dependency err: %w", err)
	}
	return nil
}

// RemoveDependency removes the dependency on dep from the issue t.
func (s *Service) RemoveDependency(ctx context.Context, t Target, dep Target) error {
	if err := s.port.RemoveIssueDependency(ctx, t.Owner, t.Repo, t.Index, dep); err != nil {
		return fmt.Errorf("remove issue dependency err: %w", err)
	}
	return nil
}
