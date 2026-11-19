// SPDX-License-Identifier: GPL-3.0-or-later

package pull

import (
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/list"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/model"
)

// The helpers below render a bounded pull request list from its use case as
// the response envelope its tool has always returned.

func pullListPage(b list.Bounded[*model.PullRequest], err error) (model.PullRequestPage, error) {
	if err != nil {
		return model.PullRequestPage{}, err
	}
	return model.PullRequestPage{PullRequests: b.Items, PageMeta: model.PageMetaOf(b)}, nil
}

func reviewPage(b list.Bounded[*model.PullReview], err error) (model.PullReviewPage, error) {
	if err != nil {
		return model.PullReviewPage{}, err
	}
	return model.PullReviewPage{Reviews: b.Items, PageMeta: model.PageMetaOf(b)}, nil
}

// reviewCommentPage renders one page of a review's inline comments. The
// endpoint has never reported a total, so the total is not shown, and an empty
// page is an empty list, never null.
func reviewCommentPage(b list.Bounded[*model.PullReviewComment], err error) (model.PullReviewCommentPage, error) {
	if err != nil {
		return model.PullReviewCommentPage{}, err
	}
	rows := b.Items
	if rows == nil {
		rows = []*model.PullReviewComment{}
	}
	meta := model.PageMetaOf(b)
	meta.TotalCount = nil
	return model.PullReviewCommentPage{Comments: rows, PageMeta: meta}, nil
}

func filesPage(b list.Bounded[*model.ChangedFile], err error) (model.ChangedFilePage, error) {
	if err != nil {
		return model.ChangedFilePage{}, err
	}
	return model.ChangedFilePage{Files: b.Items, PageMeta: model.PageMetaOf(b)}, nil
}
