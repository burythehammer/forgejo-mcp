// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"fmt"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
)

// Kind is the kind of index a read is willing to return. Forgejo gives issues
// and pull requests one index sequence, so a read can ask for one kind and be
// refused the other.
type Kind string

const (
	// KindAny accepts an index of either kind. It is the zero value.
	KindAny Kind = ""
	// KindIssues accepts only an issue; a pull request's index is refused.
	KindIssues Kind = "issues"
	// KindPulls accepts only a pull request; an issue's index is refused.
	KindPulls Kind = "pulls"
)

// ParseKind accepts "", "issues" or "pulls" and rejects any other value.
func ParseKind(s string) (Kind, error) {
	switch k := Kind(s); k {
	case KindAny, KindIssues, KindPulls:
		return k, nil
	}
	return KindAny, fmt.Errorf("invalid type %q: must be \"issues\" or \"pulls\"", s)
}

// KindError is returned by Read when the index is of the kind the read refuses.
type KindError struct {
	Index int64
	// Want is the kind the read asked for; the index is the other kind.
	Want Kind
}

func (e *KindError) Error() string {
	if e.Want == KindPulls {
		return fmt.Sprintf("index %d is an issue, not a pull request", e.Index)
	}
	return fmt.Sprintf("index %d is a pull request, not an issue", e.Index)
}

// ReadOptions says what a read of one issue carries beyond the issue itself.
// The zero value is the issue alone, which is what the get_issue_by_index tool
// returns; its comments are read through list_issue_comments.
type ReadOptions struct {
	// Kind, when not KindAny, refuses an index of the other kind with a *KindError.
	Kind Kind
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
	isPull := issue.PullRequest != nil
	if opt.Kind == KindIssues && isPull {
		return IssueRead{}, &KindError{Index: t.Index, Want: KindIssues}
	}
	if opt.Kind == KindPulls && !isPull {
		return IssueRead{}, &KindError{Index: t.Index, Want: KindPulls}
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
