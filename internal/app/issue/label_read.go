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
	Labels []*model.Label
	More   bool
	Total  *int
}

// LabelList is one label list read: the labels, each with its scope, and what
// the server reported past the page of the catalogue it paged. Page and Limit
// are the paging the read used, after the default and the ceiling.
type LabelList struct {
	Labels []model.ScopedLabel
	More   bool
	Total  *int
	Page   int
	Limit  int
}

// Envelope is the response of the label list tools: the labels with the paging
// used. HasNext is the server's own report of rows past the page, and TotalCount
// is the server's grand total when it reported one.
func (l LabelList) Envelope() model.ScopedLabelPage {
	return model.ScopedLabelPage{
		Labels: l.Labels,
		PageMeta: model.PageMeta{
			PageWindow: model.PageWindow{Page: l.Page, Limit: l.Limit, Count: len(l.Labels)},
			PageNext:   model.PageNext{HasNext: l.More},
			PageTotal:  model.PageTotal{TotalCount: l.Total},
		},
	}
}

// ReadRepoLabels is the one read behind the list_repo_labels tool and the
// forgejo://repo/{owner}/{repo}/labels resource. IncludeOrg appends the owner's
// organization labels after the repository's own; the tool asks for them by
// default and the resource does not. More and Total describe the repository's
// own rows only, because the organization's labels are not paged here.
func (s *Service) ReadRepoLabels(ctx context.Context, in RepoLabelsInput) (LabelList, error) {
	var page LabelPage
	labels, used, err := list.Window(ctx, s.port, list.Paging(in.Paging), labelsDefaultLimit,
		func(ctx context.Context, p list.Paging) ([]*model.Label, error) {
			got, err := s.port.ListRepoLabelsPage(ctx, in.Owner, in.Repo, Paging(p))
			if err != nil {
				return nil, fmt.Errorf("list repo labels err: %w", err)
			}
			page = got
			return got.Labels, nil
		})
	if err != nil {
		return LabelList{}, err
	}
	merged := make([]model.ScopedLabel, 0, len(labels))
	for _, l := range labels {
		merged = append(merged, model.ScopedLabel{Label: *l, Scope: "repo"})
	}
	if in.IncludeOrg {
		orgs, err := s.ReadOrgLabels(ctx, in.Owner, in.Paging)
		// A user owner has no organization labels; the repository already read.
		if err != nil && !errors.Is(err, ErrOrgNotFound) {
			return LabelList{}, err
		}
		merged = append(merged, orgs.Labels...)
	}
	return LabelList{Labels: merged, More: page.More, Total: page.Total, Page: used.Page, Limit: used.Limit}, nil
}

// ReadOrgLabels is the one read behind the list_org_labels tool and the
// forgejo://org/{org}/labels resource. Each label is tagged with scope "org". A
// page below 1 reads as page 1 and a limit below 1 as the default of 100.
func (s *Service) ReadOrgLabels(ctx context.Context, org string, p Paging) (LabelList, error) {
	if p.Page < 1 {
		p.Page = 1
	}
	var page LabelPage
	labels, used, err := list.Window(ctx, s.port, list.Paging(p), labelsDefaultLimit,
		func(ctx context.Context, q list.Paging) ([]*model.Label, error) {
			got, err := s.port.ListOrgLabelsPage(ctx, org, Paging(q))
			if err != nil {
				return nil, fmt.Errorf("list org labels err: %w", err)
			}
			page = got
			return got.Labels, nil
		})
	if err != nil {
		return LabelList{}, err
	}
	out := make([]model.ScopedLabel, 0, len(labels))
	for _, l := range labels {
		out = append(out, model.ScopedLabel{Label: *l, Scope: "org"})
	}
	return LabelList{Labels: out, More: page.More, Total: page.Total, Page: used.Page, Limit: used.Limit}, nil
}
