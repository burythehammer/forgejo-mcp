// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"fmt"
	"strings"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
)

// GetMilestone returns one milestone by its ID. A missing ID is a
// forge.ErrNotFound, never an empty result.
func (s *Service) GetMilestone(ctx context.Context, owner, repo string, id int64) (*model.Milestone, error) {
	m, err := s.port.GetMilestone(ctx, owner, repo, id)
	if err != nil {
		return nil, fmt.Errorf("get milestone err: %w", err)
	}
	return m, nil
}

// CreateMilestone creates an open milestone. The title is required.
func (s *Service) CreateMilestone(ctx context.Context, owner, repo string, in MilestoneNew) (*model.Milestone, error) {
	if strings.TrimSpace(in.Title) == "" {
		return nil, fmt.Errorf("create_milestone: title must be provided")
	}
	m, err := s.port.CreateMilestone(ctx, owner, repo, in)
	if err != nil {
		return nil, fmt.Errorf("create milestone err: %w", err)
	}
	return m, nil
}

// EditMilestone changes the supplied fields of a milestone. A nil field is left
// alone; an empty title counts as not supplied. State is "open" or "closed",
// which is how a milestone is closed or reopened.
func (s *Service) EditMilestone(ctx context.Context, owner, repo string, id int64, in MilestoneEdit) (*model.Milestone, error) {
	edit, err := milestoneEdit(in)
	if err != nil {
		return nil, err
	}
	m, err := s.port.EditMilestone(ctx, owner, repo, id, edit)
	if err != nil {
		return nil, fmt.Errorf("edit milestone err: %w", err)
	}
	return m, nil
}

// milestoneEdit validates an edit and returns the fields to send. An empty
// title is treated as not supplied, so a request with no usable field, the
// nil-everything case included, fails once, at the end.
func milestoneEdit(in MilestoneEdit) (MilestoneEdit, error) {
	out := MilestoneEdit{Tool: in.Tool, Description: in.Description, Due: in.Due}
	if in.Title != nil && *in.Title != "" {
		if strings.TrimSpace(*in.Title) == "" {
			return MilestoneEdit{}, fmt.Errorf("%s: title must not be blank", in.Tool)
		}
		title := *in.Title
		out.Title = &title
	}
	if in.State != nil {
		switch *in.State {
		case "open", "closed":
		default:
			return MilestoneEdit{}, fmt.Errorf("%s: state must be open or closed", in.Tool)
		}
		state := *in.State
		out.State = &state
	}
	if out.Title == nil && out.Description == nil && out.State == nil && out.Due == nil {
		return MilestoneEdit{}, fmt.Errorf("%s: at least one of title, description, state, due_date must be provided", in.Tool)
	}
	return out, nil
}
