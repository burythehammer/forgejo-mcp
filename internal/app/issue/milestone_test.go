// SPDX-License-Identifier: GPL-3.0-or-later

package issue_test

import (
	"context"
	"testing"

	apppissue "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/issue"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
)

// milestonePort records the milestone writes the use cases send. Any other
// port method panics on the nil embed, which a milestone test never reaches.
type milestonePort struct {
	apppissue.Port
	created []apppissue.MilestoneNew
	edited  []apppissue.MilestoneEdit
}

func (p *milestonePort) CreateMilestone(_ context.Context, _, _ string, in apppissue.MilestoneNew) (*model.Milestone, error) {
	p.created = append(p.created, in)
	return &model.Milestone{ID: 1, Title: in.Title, State: "open"}, nil
}

func (p *milestonePort) EditMilestone(_ context.Context, _, _ string, id int64, in apppissue.MilestoneEdit) (*model.Milestone, error) {
	p.edited = append(p.edited, in)
	return &model.Milestone{ID: id, State: "open"}, nil
}

func strPtr(s string) *string { return &s }

func TestCreateMilestone_RequiresATitle(t *testing.T) {
	t.Parallel()
	p := &milestonePort{}
	s := apppissue.New(p)
	if _, err := s.CreateMilestone(context.Background(), "o", "r", apppissue.MilestoneNew{Title: " \t "}); err == nil {
		t.Fatal("blank title: want an error")
	}
	if len(p.created) != 0 {
		t.Fatalf("port called for a rejected create: %+v", p.created)
	}
	if _, err := s.CreateMilestone(context.Background(), "o", "r", apppissue.MilestoneNew{Title: "v2", Description: "d"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(p.created) != 1 || p.created[0].Title != "v2" || p.created[0].Description != "d" {
		t.Fatalf("created = %+v", p.created)
	}
}

func TestEditMilestone_RejectsAnEditThatChangesNothing(t *testing.T) {
	t.Parallel()
	cases := map[string]apppissue.MilestoneEdit{
		"no fields":   {Tool: "edit_milestone"},
		"empty title": {Tool: "edit_milestone", Title: strPtr("")},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p := &milestonePort{}
			if _, err := apppissue.New(p).EditMilestone(context.Background(), "o", "r", 1, in); err == nil {
				t.Fatal("want an error")
			}
			if len(p.edited) != 0 {
				t.Fatalf("port called: %+v", p.edited)
			}
		})
	}
}

func TestEditMilestone_RejectsABadStateAndABlankTitle(t *testing.T) {
	t.Parallel()
	for name, in := range map[string]apppissue.MilestoneEdit{
		"state":       {Tool: "edit_milestone", State: strPtr("archived")},
		"blank title": {Tool: "edit_milestone", Title: strPtr("  "), State: strPtr("closed")},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			p := &milestonePort{}
			if _, err := apppissue.New(p).EditMilestone(context.Background(), "o", "r", 1, in); err == nil {
				t.Fatal("want an error")
			}
			if len(p.edited) != 0 {
				t.Fatalf("port called: %+v", p.edited)
			}
		})
	}
}

func TestEditMilestone_ClosesWithAnEmptyDescriptionKept(t *testing.T) {
	t.Parallel()
	p := &milestonePort{}
	_, err := apppissue.New(p).EditMilestone(context.Background(), "o", "r", 7, apppissue.MilestoneEdit{
		Tool: "edit_milestone", State: strPtr("closed"), Description: strPtr(""),
	})
	if err != nil {
		t.Fatalf("edit: %v", err)
	}
	if len(p.edited) != 1 {
		t.Fatalf("edited = %+v", p.edited)
	}
	got := p.edited[0]
	if got.State == nil || *got.State != "closed" {
		t.Fatalf("state = %v, want closed", got.State)
	}
	if got.Description == nil || *got.Description != "" {
		t.Fatalf("description = %v, want an explicit empty string", got.Description)
	}
	if got.Title != nil {
		t.Fatalf("title = %v, want it left alone", *got.Title)
	}
}
