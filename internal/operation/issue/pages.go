// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/list"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/model"
)

// The helpers below render a bounded list from its use case as the response
// envelope its tool has always returned. The rows and the continuation
// metadata both come from the list.Bounded, so the envelope cannot disagree
// with the page it was built from.

func issueListPage(b list.Bounded[*model.Issue], err error) (model.IssuePage, error) {
	if err != nil {
		return model.IssuePage{}, err
	}
	return model.IssuePage{Issues: b.Items, PageMeta: model.PageMetaOf(b)}, nil
}

func issueSearchPage(b list.Bounded[*model.Issue], err error) (model.IssueSearchPage, error) {
	if err != nil {
		return model.IssueSearchPage{}, err
	}
	return model.IssueSearchPage{Issues: b.Items, PageMeta: model.PageMetaOf(b)}, nil
}

func commentPage(b list.Bounded[*model.Comment], err error) (model.CommentPage, error) {
	if err != nil {
		return model.CommentPage{}, err
	}
	return model.CommentPage{Comments: b.Items, PageMeta: model.PageMetaOf(b)}, nil
}

func dependencyPage(b list.Bounded[*model.Issue], err error) (model.DependencyPage, error) {
	if err != nil {
		return model.DependencyPage{}, err
	}
	return model.DependencyPage{Issues: b.Items, PageMeta: model.PageMetaOf(b)}, nil
}

func milestonePage(b list.Bounded[*model.Milestone], err error) (model.MilestonePage, error) {
	if err != nil {
		return model.MilestonePage{}, err
	}
	return model.MilestonePage{Milestones: b.Items, PageMeta: model.PageMetaOf(b)}, nil
}

// scopedLabelPage renders a label list. The total and has_next describe the
// repository's own rows; Count includes the organization's labels merged in.
func scopedLabelPage(b list.Bounded[model.ScopedLabel]) model.ScopedLabelPage {
	return model.ScopedLabelPage{Labels: b.Items, PageMeta: model.PageMetaOf(b)}
}
