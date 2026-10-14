// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"fmt"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/list"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
)

// Comments lists an issue's (or pull request's) comments, within the optional
// since and before window.
func (s *Service) Comments(ctx context.Context, in CommentsInput) (model.CommentPage, error) {
	var c CommentList
	if in.Since != "" {
		t, err := time.Parse(time.RFC3339, in.Since)
		if err != nil {
			return model.CommentPage{}, fmt.Errorf("invalid since time format (expected RFC3339): %w", err)
		}
		c.Since = t
	}
	if in.Before != "" {
		t, err := time.Parse(time.RFC3339, in.Before)
		if err != nil {
			return model.CommentPage{}, fmt.Errorf("invalid before time format (expected RFC3339): %w", err)
		}
		c.Before = t
	}
	page, err := runPage(ctx, s.port, list.Paging(in.Paging), defaultLimit,
		func(ctx context.Context, p list.Paging) ([]*model.Comment, error) {
			q := c
			q.Paging = Paging(p)
			comments, err := s.port.ListIssueComments(ctx, in.Owner, in.Repo, in.Index, q)
			if err != nil {
				return nil, fmt.Errorf("list issue comments err: %w", err)
			}
			return comments, nil
		})
	if err != nil {
		return model.CommentPage{}, err
	}
	return model.CommentPage{
		Comments: page.Items, PageMeta: model.PageMetaOf(page),
	}, nil
}

// Comment returns one comment by its ID.
func (s *Service) Comment(ctx context.Context, owner, repo string, id int64) (*model.Comment, error) {
	comment, err := s.port.GetIssueComment(ctx, owner, repo, id)
	if err != nil {
		return nil, fmt.Errorf("get issue comment err: %w", err)
	}
	return comment, nil
}

// CreateComment comments on an issue or pull request.
func (s *Service) CreateComment(ctx context.Context, t Target, body string) (*model.Comment, error) {
	comment, err := s.port.CreateIssueComment(ctx, t.Owner, t.Repo, t.Index, body)
	if err != nil {
		return nil, fmt.Errorf("create issue comment err: %w", err)
	}
	return comment, nil
}

// EditComment replaces the body of a comment.
func (s *Service) EditComment(ctx context.Context, owner, repo string, id int64, body string) (*model.Comment, error) {
	comment, err := s.port.EditIssueComment(ctx, owner, repo, id, body)
	if err != nil {
		return nil, fmt.Errorf("edit issue comment err: %w", err)
	}
	return comment, nil
}

// DeleteComment deletes a comment by its ID.
func (s *Service) DeleteComment(ctx context.Context, owner, repo string, id int64) error {
	if err := s.port.DeleteIssueComment(ctx, owner, repo, id); err != nil {
		return fmt.Errorf("delete issue comment err: %w", err)
	}
	return nil
}
