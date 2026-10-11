// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"fmt"
	"strings"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
)

// Dependencies lists the issues that an issue depends on, one page at a time.
func (s *Service) Dependencies(ctx context.Context, t Target, p Paging) (model.DependencyPage, error) {
	issues, err := s.port.ListIssueDependencies(ctx, t.Owner, t.Repo, t.Index, p)
	if err != nil {
		return model.DependencyPage{}, fmt.Errorf("list issue dependencies err: %w", err)
	}
	return model.DependencyPage{Page: p.Page, Limit: p.Limit, Issues: issues}, nil
}

// Dependents lists the issues that depend on an issue, one page at a time.
func (s *Service) Dependents(ctx context.Context, t Target, p Paging) (model.DependencyPage, error) {
	issues, err := s.port.ListIssueDependents(ctx, t.Owner, t.Repo, t.Index, p)
	if err != nil {
		return model.DependencyPage{}, fmt.Errorf("list issue dependents err: %w", err)
	}
	return model.DependencyPage{Page: p.Page, Limit: p.Limit, Issues: issues}, nil
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
