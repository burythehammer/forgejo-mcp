// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"fmt"
	"strings"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
)

// ReplaceBodyLineInput replaces one piece of an issue's body. Old is matched as
// a case-sensitive substring, counted without overlap, and must occur exactly
// once. New takes its place; it may be empty.
type ReplaceBodyLineInput struct {
	Target
	Old, New string
}

// ReplaceBodyLine replaces the single occurrence of Old in an issue's body with
// New. It is one read and one write. Any refusal (empty Old, a pull request
// index, zero matches, more than one match) happens before the write, so the
// body is left untouched.
//
// Forgejo's issue edit has no compare-and-swap, so an edit made between the
// read and the write can be lost. The returned issue's UpdatedAt lets a caller
// see that a write happened; this use case does not try to hide the race.
func (s *Service) ReplaceBodyLine(ctx context.Context, in ReplaceBodyLineInput) (*model.Issue, error) {
	if in.Old == "" {
		return nil, fmt.Errorf("old must not be empty: an empty text occurs everywhere")
	}
	issue, err := s.port.GetIssue(ctx, in.Owner, in.Repo, in.Index)
	if err != nil {
		return nil, fmt.Errorf("get issue err: %w", err)
	}
	if issue.PullRequest != nil {
		return nil, fmt.Errorf("#%d is a pull request; replace_issue_body_line edits issues only, nothing changed", in.Index)
	}
	switch n := strings.Count(issue.Body, in.Old); {
	case n == 0:
		return nil, fmt.Errorf("old text not found in the issue body; nothing changed")
	case n > 1:
		return nil, fmt.Errorf("old text occurs %d times in the issue body; it must occur exactly once, so nothing changed. Make old more specific", n)
	}
	body := strings.Replace(issue.Body, in.Old, in.New, 1)
	edited, err := s.port.EditIssue(ctx, in.Owner, in.Repo, in.Index, IssueEdit{Body: &body})
	if err != nil {
		return nil, fmt.Errorf("update issue err: %w", err)
	}
	return edited, nil
}
