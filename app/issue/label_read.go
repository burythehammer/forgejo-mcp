// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"fmt"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
)

// LabelPage is one page of a label catalogue as the server pages it. More is
// true when the server reported rows past this page; Total is the server's
// count of all rows, known only when the server reported one.
type LabelPage struct {
	Labels []*model.Label
	More   bool
	Total  Total
}

// LabelList is one label list read: the labels, each with its scope, and what
// the server reported past the page of the catalogue it paged.
type LabelList struct {
	Labels []model.ScopedLabel
	More   bool
	Total  Total
}

// ReadRepoLabels is the one read behind the list_repo_labels tool and the
// forgejo://repo/{owner}/{repo}/labels resource. IncludeOrg appends the owner's
// organization labels after the repository's own; the tool asks for them by
// default and the resource does not. More and Total describe the repository's
// own rows only, because the organization's labels are not paged here.
func (s *Service) ReadRepoLabels(ctx context.Context, in RepoLabelsInput) (LabelList, error) {
	page, err := s.port.ListRepoLabelsPage(ctx, in.Owner, in.Repo, in.Paging)
	if err != nil {
		return LabelList{}, fmt.Errorf("list repo labels err: %w", err)
	}
	merged := make([]model.ScopedLabel, 0, len(page.Labels))
	for _, l := range page.Labels {
		merged = append(merged, model.ScopedLabel{Label: *l, Scope: "repo"})
	}
	if in.IncludeOrg {
		orgs, err := s.ReadOrgLabels(ctx, in.Owner, in.Paging)
		if err != nil {
			return LabelList{}, err
		}
		merged = append(merged, orgs.Labels...)
	}
	return LabelList{Labels: merged, More: page.More, Total: page.Total}, nil
}

// ReadOrgLabels is the one read behind the list_org_labels tool and the
// forgejo://org/{org}/labels resource. Each label is tagged with scope "org". A
// page below 1 reads as page 1 and a limit below 1 as the default of 100.
func (s *Service) ReadOrgLabels(ctx context.Context, org string, p Paging) (LabelList, error) {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.Limit < 1 {
		p.Limit = 100
	}
	page, err := s.port.ListOrgLabelsPage(ctx, org, p)
	if err != nil {
		return LabelList{}, fmt.Errorf("list org labels err: %w", err)
	}
	out := make([]model.ScopedLabel, 0, len(page.Labels))
	for _, l := range page.Labels {
		out = append(out, model.ScopedLabel{Label: *l, Scope: "org"})
	}
	return LabelList{Labels: out, More: page.More, Total: page.Total}, nil
}
