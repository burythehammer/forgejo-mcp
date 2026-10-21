// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"fmt"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
)

// ReadOptions says what a read of one issue carries beyond the issue itself.
// The zero value is the issue alone, which is what the get_issue_by_index tool
// returns; its comments are read through list_issue_comments.
type ReadOptions struct {
	// CommentCap, when above zero, also reads the issue's first CommentCap
	// comments, in the order Forgejo returns them. The issue resource asks for
	// this, with the cap that bounds its embedded thread.
	CommentCap int
}

// IssueRead is one issue read with the options it asked for.
type IssueRead struct {
	Issue *model.Issue
	// Comments holds at most CommentCap comments. It is nil when none were asked for.
	Comments []*model.Comment
	// CommentsTruncated is true when the issue has more comments than CommentCap.
	CommentsTruncated bool
}

// Read returns one issue and, when opt asks for them, its recent comments. It
// is the one read behind the get_issue_by_index tool and the forgejo://
// issue resource; the two adapters differ only in what they ask for and how
// they encode the result.
func (s *Service) Read(ctx context.Context, t Target, opt ReadOptions) (IssueRead, error) {
	issue, err := s.port.GetIssue(ctx, t.Owner, t.Repo, t.Index)
	if err != nil {
		return IssueRead{}, fmt.Errorf("get issue err: %w", err)
	}
	read := IssueRead{Issue: issue}
	if opt.CommentCap <= 0 {
		return read, nil
	}
	// One comment more than the cap, so that "more than the cap" is visible
	// without a second call.
	comments, err := s.port.ListIssueComments(ctx, t.Owner, t.Repo, t.Index, CommentList{
		Paging: Paging{Limit: opt.CommentCap + 1},
	})
	if err != nil {
		return IssueRead{}, fmt.Errorf("list issue comments err: %w", err)
	}
	if len(comments) > opt.CommentCap {
		read.CommentsTruncated = true
		comments = comments[:opt.CommentCap]
	}
	read.Comments = comments
	return read, nil
}
