// SPDX-License-Identifier: GPL-3.0-or-later

package pull

import (
	"context"
	"fmt"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/model"
)

// ReadOptions says what a read of one pull request carries beyond the pull
// request itself. The zero value is the pull request alone, which is what the
// get_pull_request_by_index tool returns. A cap above zero also reads that many
// of the discussion thread or of the reviews, in the order Forgejo returns
// them: the pull-request resource asks for both, with the cap that bounds its
// embedded lists.
type ReadOptions struct {
	CommentCap int
	ReviewCap  int
}

// PullRead is one pull request read with the options it asked for.
type PullRead struct {
	PR *model.PullRequest
	// Comments holds at most CommentCap comments. It is nil when none were asked for.
	Comments []*model.Comment
	// CommentsTruncated is true when the thread has more comments than CommentCap.
	CommentsTruncated bool
	// Reviews holds at most ReviewCap reviews. It is nil when none were asked for.
	Reviews []*model.PullReview
	// ReviewsTruncated is true when there are more reviews than ReviewCap.
	ReviewsTruncated bool
	// ReviewsTotal is the number of the pull request's reviews, not only the
	// ones held. It is valid only when ReviewsTotalKnown: when the reviews fit
	// under the cap it is exact, and when they do not it is the server's total,
	// which Forgejo may not report.
	ReviewsTotal      int
	ReviewsTotalKnown bool
}

// Read returns one pull request and, when opt asks for them, its discussion
// thread and reviews. It is the one read behind the get_pull_request_by_index
// tool and the forgejo:// pull-request resource; the two adapters differ only
// in what they ask for and how they encode the result.
func (s *Service) Read(ctx context.Context, in PR, opt ReadOptions) (PullRead, error) {
	pr, err := s.port.GetPullRequest(ctx, in.Owner, in.Repo, in.Index)
	if err != nil {
		return PullRead{}, fmt.Errorf("get pull request err: %w", err)
	}
	read := PullRead{PR: pr}

	if opt.CommentCap > 0 {
		// One comment more than the cap, so that "more than the cap" is visible
		// without a second call.
		comments, err := s.port.ListPullRequestComments(ctx, in.Owner, in.Repo, in.Index, Paging{Limit: opt.CommentCap + 1})
		if err != nil {
			return PullRead{}, fmt.Errorf("list pull request comments err: %w", err)
		}
		if len(comments) > opt.CommentCap {
			read.CommentsTruncated = true
			comments = comments[:opt.CommentCap]
		}
		read.Comments = comments
	}

	if opt.ReviewCap > 0 {
		page, err := s.port.ListPullReviews(ctx, in.Owner, in.Repo, in.Index, Paging{Limit: opt.ReviewCap + 1})
		if err != nil {
			return PullRead{}, fmt.Errorf("list pull reviews err: %w", err)
		}
		reviews := page.Reviews
		read.ReviewsTotal, read.ReviewsTotalKnown = len(reviews), true
		if len(reviews) > opt.ReviewCap {
			read.ReviewsTruncated = true
			reviews = reviews[:opt.ReviewCap]
			read.ReviewsTotal, read.ReviewsTotalKnown = page.Total, page.TotalKnown
		}
		read.Reviews = reviews
	}
	return read, nil
}
