// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"errors"
	"fmt"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/list"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
)

// labelsDefaultLimit is the page size a repository or organization label
// list uses when the caller gives none.
const labelsDefaultLimit = 100

// LabelPage is one page of a label catalogue as the server pages it. More is
// true when the server reported rows past this page; Total is the server's
// count of all rows, known only when the server reported one.
type LabelPage struct {
	Labels []*model.CatalogLabel
	More   bool
	Total  *int
}

// ReadRepoLabels is the one read behind the list_repo_labels tool and the
// forgejo://repo/{owner}/{repo}/labels resource. IncludeOrg appends the owner's
// organization labels after the repository's own; the tool asks for them by
// default and the resource does not. HasNext and Total describe the
// repository's own rows only, because the organization's labels are not paged
// here. Each label carries its scope.
func (s *Service) ReadRepoLabels(ctx context.Context, in RepoLabelsInput) (list.Bounded[model.ScopedLabel], error) {
	bounded, err := list.Window(ctx, s.port, list.NewRequest(list.Paging(in.Paging), labelsDefaultLimit),
		func(ctx context.Context, p list.Paging) (list.Batch[*model.CatalogLabel], error) {
			got, err := s.port.ListRepoLabelsPage(ctx, in.Owner, in.Repo, Paging(p))
			if err != nil {
				return list.Batch[*model.CatalogLabel]{}, fmt.Errorf("list repo labels err: %w", err)
			}
			return list.Batch[*model.CatalogLabel]{Items: got.Labels, Total: got.Total, More: got.More}, nil
		})
	if err != nil {
		return list.Bounded[model.ScopedLabel]{}, err
	}
	repo := list.Map(bounded, func(l *model.CatalogLabel) model.ScopedLabel {
		return model.ScopedLabel{CatalogLabel: *l, Scope: "repo"}
	})
	if !in.IncludeOrg {
		return repo, nil
	}
	orgs, err := s.ReadOrgLabels(ctx, in.Owner, in.Paging)
	// A user owner has no organization labels; the repository already read.
	if err != nil && !errors.Is(err, ErrOrgNotFound) {
		return list.Bounded[model.ScopedLabel]{}, err
	}
	return repo.Extend(orgs.Items...), nil
}

// ReadOrgLabels is the one read behind the list_org_labels tool and the
// forgejo://org/{org}/labels resource. Each label is tagged with scope "org". A
// limit below 1 takes the default of 100; the page is passed on as given.
func (s *Service) ReadOrgLabels(ctx context.Context, org string, p Paging) (list.Bounded[model.ScopedLabel], error) {
	bounded, err := list.Window(ctx, s.port, list.NewRequest(list.Paging(p), labelsDefaultLimit),
		func(ctx context.Context, q list.Paging) (list.Batch[*model.CatalogLabel], error) {
			got, err := s.port.ListOrgLabelsPage(ctx, org, Paging(q))
			if err != nil {
				return list.Batch[*model.CatalogLabel]{}, fmt.Errorf("list org labels err: %w", err)
			}
			return list.Batch[*model.CatalogLabel]{Items: got.Labels, Total: got.Total, More: got.More}, nil
		})
	if err != nil {
		return list.Bounded[model.ScopedLabel]{}, err
	}
	return list.Map(bounded, func(l *model.CatalogLabel) model.ScopedLabel {
		return model.ScopedLabel{CatalogLabel: *l, Scope: "org"}
	}), nil
}
