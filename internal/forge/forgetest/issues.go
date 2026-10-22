// SPDX-License-Identifier: GPL-3.0-or-later

package forgetest

import (
	"fmt"
	"testing"
	"time"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// RunIssues runs the issue contract against the implementation that
// newHarness builds. newHarness is called once per case.
func RunIssues(t *testing.T, newHarness func(t *testing.T) Harness) {
	cases := []struct {
		name string
		run  func(t *testing.T, h Harness)
	}{
		{"get unknown issue is not found", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			_, err := h.Port.GetIssue(ctx, Owner, Repo, 99)
			wantNotFound(t, err)
			_, err = h.Port.GetIssue(ctx, "nobody", "nothing", 1)
			wantNotFound(t, err)
		}},
		{"created issue reads back", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			ms := h.Seed.SeedMilestone(Owner, Repo, "v1")
			label := h.Seed.SeedIssueLabel(Owner, Repo, "bug")
			a := newIssue(t, h, "first")
			b, err := h.Port.CreateIssue(ctx, Owner, Repo, forgejo_sdk.CreateIssueOption{
				Title: "second", Body: "details", Ref: "refs/heads/dev", Assignees: []string{"bob"}, Milestone: ms, Labels: []int64{label},
			})
			must(t, err)
			if a.Index == b.Index {
				t.Fatalf("indices must differ, both %d", a.Index)
			}
			got, err := h.Port.GetIssue(ctx, Owner, Repo, b.Index)
			must(t, err)
			if got.Title != "second" || got.Body != "details" || got.State != forgejo_sdk.StateOpen || got.Ref != "refs/heads/dev" {
				t.Fatalf("got title=%q body=%q state=%q ref=%q", got.Title, got.Body, got.State, got.Ref)
			}
			if len(got.Assignees) != 1 || got.Assignees[0].UserName != "bob" {
				t.Fatalf("assignees: %+v", got.Assignees)
			}
			if got.Milestone == nil || got.Milestone.Title != "v1" {
				t.Fatalf("milestone: %+v", got.Milestone)
			}
			if len(got.Labels) != 1 || got.Labels[0].Name != "bug" {
				t.Fatalf("labels: %+v", got.Labels)
			}
		}},
		{"create without a title is rejected", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			_, err := h.Port.CreateIssue(ctx, Owner, Repo, forgejo_sdk.CreateIssueOption{Body: "x"})
			if err == nil {
				t.Fatal("want an error")
			}
		}},
		{"create in an unknown repository is not found", func(t *testing.T, h Harness) {
			_, err := h.Port.CreateIssue(ctx, "nobody", "nothing", forgejo_sdk.CreateIssueOption{Title: "t"})
			wantNotFound(t, err)
		}},
		{"edit changes only what is given", func(t *testing.T, h Harness) {
			iss := newIssue(t, h, "orig")
			body := "new body"
			due := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
			got, err := h.Port.EditIssue(ctx, Owner, Repo, iss.Index, forgejo_sdk.EditIssueOption{
				Body: &body, Assignees: []string{"carol"}, Deadline: &due,
			})
			must(t, err)
			if got.Title != "orig" || got.Body != "new body" {
				t.Fatalf("title=%q body=%q", got.Title, got.Body)
			}
			if len(got.Assignees) != 1 || got.Assignees[0].UserName != "carol" {
				t.Fatalf("assignees: %+v", got.Assignees)
			}
			if got.Deadline == nil || !got.Deadline.Equal(due) {
				t.Fatalf("deadline: %v", got.Deadline)
			}
			remove := true
			got, err = h.Port.EditIssue(ctx, Owner, Repo, iss.Index, forgejo_sdk.EditIssueOption{RemoveDeadline: &remove, Assignees: []string{}})
			must(t, err)
			if got.Deadline != nil || len(got.Assignees) != 0 {
				t.Fatalf("deadline=%v assignees=%+v", got.Deadline, got.Assignees)
			}
		}},
		{"edit unknown issue is not found", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			_, err := h.Port.EditIssue(ctx, Owner, Repo, 99, forgejo_sdk.EditIssueOption{Title: "x"})
			wantNotFound(t, err)
		}},
		{"closing and reopening an issue", func(t *testing.T, h Harness) {
			iss := newIssue(t, h, "c")
			closed := forgejo_sdk.StateClosed
			got, err := h.Port.EditIssue(ctx, Owner, Repo, iss.Index, forgejo_sdk.EditIssueOption{State: &closed})
			must(t, err)
			if got.State != forgejo_sdk.StateClosed || got.Closed == nil {
				t.Fatalf("after close: state=%q closed=%v", got.State, got.Closed)
			}
			open := forgejo_sdk.StateOpen
			got, err = h.Port.EditIssue(ctx, Owner, Repo, iss.Index, forgejo_sdk.EditIssueOption{State: &open})
			must(t, err)
			if got.State != forgejo_sdk.StateOpen {
				t.Fatalf("after reopen: state=%q", got.State)
			}
		}},
		{"labels are replaced and cleared", func(t *testing.T, h Harness) {
			iss := newIssue(t, h, "l")
			a := h.Seed.SeedIssueLabel(Owner, Repo, "a")
			b := h.Seed.SeedIssueLabel(Owner, Repo, "b")
			must(t, h.Port.ReplaceIssueLabels(ctx, Owner, Repo, iss.Index, []int64{a, b}))
			wantLabels(t, h, iss.Index, "a", "b")
			must(t, h.Port.ReplaceIssueLabels(ctx, Owner, Repo, iss.Index, []int64{b}))
			wantLabels(t, h, iss.Index, "b")
			must(t, h.Port.ClearIssueLabels(ctx, Owner, Repo, iss.Index))
			wantLabels(t, h, iss.Index)
			wantNotFound(t, h.Port.ClearIssueLabels(ctx, Owner, Repo, 99))
		}},
		{"list defaults to open, newest first", func(t *testing.T, h Harness) {
			a := newIssue(t, h, "a")
			b := newIssue(t, h, "b")
			c := newIssue(t, h, "c")
			closeIssue(t, h, b.Index)
			got, _, err := h.Port.ListRepoIssues(ctx, Owner, Repo, forge.ListIssuesOptions{})
			must(t, err)
			wantIssueIndices(t, got, c.Index, a.Index)
		}},
		{"list filters by state, label and sorts", func(t *testing.T, h Harness) {
			a := newIssue(t, h, "a")
			b := newIssue(t, h, "b")
			closeIssue(t, h, a.Index)
			bug := h.Seed.SeedIssueLabel(Owner, Repo, "bug")
			must(t, h.Port.ReplaceIssueLabels(ctx, Owner, Repo, b.Index, []int64{bug}))

			closed, _, err := h.Port.ListRepoIssues(ctx, Owner, Repo, forge.ListIssuesOptions{ListIssueOption: forgejo_sdk.ListIssueOption{State: forgejo_sdk.StateClosed}})
			must(t, err)
			wantIssueIndices(t, closed, a.Index)

			all, _, err := h.Port.ListRepoIssues(ctx, Owner, Repo, forge.ListIssuesOptions{ListIssueOption: forgejo_sdk.ListIssueOption{State: forgejo_sdk.StateAll}, Sort: "oldest"})
			must(t, err)
			wantIssueIndices(t, all, a.Index, b.Index)

			labelled, _, err := h.Port.ListRepoIssues(ctx, Owner, Repo, forge.ListIssuesOptions{ListIssueOption: forgejo_sdk.ListIssueOption{State: forgejo_sdk.StateAll, Labels: []string{"bug"}}})
			must(t, err)
			wantIssueIndices(t, labelled, b.Index)
		}},
		{"list sorted by due date", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			late := time.Date(2031, 1, 1, 0, 0, 0, 0, time.UTC)
			soon := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
			a, err := h.Port.CreateIssue(ctx, Owner, Repo, forgejo_sdk.CreateIssueOption{Title: "late", Deadline: &late})
			must(t, err)
			b, err := h.Port.CreateIssue(ctx, Owner, Repo, forgejo_sdk.CreateIssueOption{Title: "soon", Deadline: &soon})
			must(t, err)
			got, _, err := h.Port.ListRepoIssues(ctx, Owner, Repo, forge.ListIssuesOptions{Sort: "nearduedate"})
			must(t, err)
			wantIssueIndices(t, got, b.Index, a.Index)
		}},
		{"list in an unknown repository is not found", func(t *testing.T, h Harness) {
			_, _, err := h.Port.ListRepoIssues(ctx, "nobody", "nothing", forge.ListIssuesOptions{})
			wantNotFound(t, err)
			_, _, err = h.Port.ListRepoIssues(ctx, "nobody", "nothing", forge.ListIssuesOptions{Sort: "latest"})
			wantNotFound(t, err)
		}},
		{"list paginates and reports what follows", func(t *testing.T, h Harness) {
			var idx []int64
			for i := 0; i < 5; i++ {
				idx = append(idx, newIssue(t, h, fmt.Sprint("i", i)).Index)
			}
			opt := func(p int) forge.ListIssuesOptions {
				return forge.ListIssuesOptions{ListIssueOption: forgejo_sdk.ListIssueOption{ListOptions: forgejo_sdk.ListOptions{Page: p, PageSize: 2}}}
			}
			p1, info1, err := h.Port.ListRepoIssues(ctx, Owner, Repo, opt(1))
			must(t, err)
			wantIssueIndices(t, p1, idx[4], idx[3])
			if !info1.HasNext || !info1.TotalKnown || info1.TotalCount != 5 {
				t.Fatalf("page 1 info: %+v", info1)
			}
			p3, info3, err := h.Port.ListRepoIssues(ctx, Owner, Repo, opt(3))
			must(t, err)
			wantIssueIndices(t, p3, idx[0])
			if info3.HasNext {
				t.Fatalf("page 3 info: %+v", info3)
			}
		}},
		{"search spans an owner's repositories", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, "other")
			h.Seed.AddRepo("elsewhere", "x")
			a := newIssue(t, h, "needle here")
			b, err := h.Port.CreateIssue(ctx, Owner, "other", forgejo_sdk.CreateIssueOption{Title: "also a needle"})
			must(t, err)
			_, err = h.Port.CreateIssue(ctx, "elsewhere", "x", forgejo_sdk.CreateIssueOption{Title: "needle but foreign"})
			must(t, err)
			_ = newIssue(t, h, "hay")

			got, _, err := h.Port.SearchIssues(ctx, forgejo_sdk.ListIssueOption{Owner: Owner, KeyWord: "needle"})
			must(t, err)
			if len(got) != 2 {
				t.Fatalf("got %d issues, want 2: %+v", len(got), got)
			}
			seen := map[string]int64{}
			for _, i := range got {
				seen[i.Repository.Name] = i.Index
			}
			if seen[Repo] != a.Index || seen["other"] != b.Index {
				t.Fatalf("got %v", seen)
			}
		}},
		{"comments round-trip", func(t *testing.T, h Harness) {
			iss := newIssue(t, h, "c")
			c1, err := h.Port.CreateIssueComment(ctx, Owner, Repo, iss.Index, "one")
			must(t, err)
			c2, err := h.Port.CreateIssueComment(ctx, Owner, Repo, iss.Index, "two")
			must(t, err)
			if c1.ID == c2.ID {
				t.Fatalf("comment ids must differ, both %d", c1.ID)
			}
			got, err := h.Port.GetIssueComment(ctx, Owner, Repo, c1.ID)
			must(t, err)
			if got.Body != "one" || got.Poster == nil || got.Poster.UserName == "" {
				t.Fatalf("got %+v", got)
			}
			list, _, err := h.Port.ListIssueComments(ctx, Owner, Repo, iss.Index, forgejo_sdk.ListIssueCommentOptions{})
			must(t, err)
			if len(list) != 2 || list[0].Body != "one" || list[1].Body != "two" {
				t.Fatalf("list: %+v", list)
			}
			after, err := h.Port.GetIssue(ctx, Owner, Repo, iss.Index)
			must(t, err)
			if after.Comments != 2 {
				t.Fatalf("issue comment count = %d, want 2", after.Comments)
			}
		}},
		{"comment listing honours the page limit", func(t *testing.T, h Harness) {
			// The bounded read asks for cap+1 rows to learn whether more exist.
			// That only works if the adapter sends the limit through.
			iss := newIssue(t, h, "paged")
			for _, body := range []string{"one", "two", "three"} {
				if _, err := h.Port.CreateIssueComment(ctx, Owner, Repo, iss.Index, body); err != nil {
					t.Fatal(err)
				}
			}
			first, _, err := h.Port.ListIssueComments(ctx, Owner, Repo, iss.Index, forgejo_sdk.ListIssueCommentOptions{
				ListOptions: forgejo_sdk.ListOptions{Page: 1, PageSize: 2},
			})
			must(t, err)
			if len(first) != 2 || first[0].Body != "one" || first[1].Body != "two" {
				t.Fatalf("page 1 of 2: got %d comments, want [one two]", len(first))
			}
			second, _, err := h.Port.ListIssueComments(ctx, Owner, Repo, iss.Index, forgejo_sdk.ListIssueCommentOptions{
				ListOptions: forgejo_sdk.ListOptions{Page: 2, PageSize: 2},
			})
			must(t, err)
			if len(second) != 1 || second[0].Body != "three" {
				t.Fatalf("page 2 of 2: got %d comments, want [three]", len(second))
			}
		}},
		{"comment edit and delete", func(t *testing.T, h Harness) {
			iss := newIssue(t, h, "c")
			c, err := h.Port.CreateIssueComment(ctx, Owner, Repo, iss.Index, "draft")
			must(t, err)
			edited, err := h.Port.EditIssueComment(ctx, Owner, Repo, c.ID, "final")
			must(t, err)
			if edited.Body != "final" {
				t.Fatalf("edited: %+v", edited)
			}
			got, err := h.Port.GetIssueComment(ctx, Owner, Repo, c.ID)
			must(t, err)
			if got.Body != "final" {
				t.Fatalf("read back: %+v", got)
			}
			must(t, h.Port.DeleteIssueComment(ctx, Owner, Repo, c.ID))
			_, err = h.Port.GetIssueComment(ctx, Owner, Repo, c.ID)
			wantNotFound(t, err)
			wantNotFound(t, h.Port.DeleteIssueComment(ctx, Owner, Repo, c.ID))
			_, err = h.Port.EditIssueComment(ctx, Owner, Repo, c.ID, "again")
			wantNotFound(t, err)
		}},
		{"comments on unknown issues are not found", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			_, err := h.Port.CreateIssueComment(ctx, Owner, Repo, 99, "x")
			wantNotFound(t, err)
			_, _, err = h.Port.ListIssueComments(ctx, Owner, Repo, 99, forgejo_sdk.ListIssueCommentOptions{})
			wantNotFound(t, err)
			_, err = h.Port.GetIssueComment(ctx, Owner, Repo, 99)
			wantNotFound(t, err)
		}},
		{"milestones filter by state", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			h.Seed.SeedMilestone(Owner, Repo, "v1")
			done := h.Seed.SeedMilestone(Owner, Repo, "v0")
			h.Seed.CloseMilestone(Owner, Repo, done)

			open, err := h.Port.ListRepoMilestones(ctx, Owner, Repo, forgejo_sdk.ListMilestoneOption{})
			must(t, err)
			if len(open) != 1 || open[0].Title != "v1" {
				t.Fatalf("open: %+v", open)
			}
			all, err := h.Port.ListRepoMilestones(ctx, Owner, Repo, forgejo_sdk.ListMilestoneOption{State: forgejo_sdk.StateAll})
			must(t, err)
			if len(all) != 2 {
				t.Fatalf("all: %+v", all)
			}
			_, err = h.Port.ListRepoMilestones(ctx, "nobody", "nothing", forgejo_sdk.ListMilestoneOption{})
			wantNotFound(t, err)
		}},
		{"milestone create, edit, close, reopen and get", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			due := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
			created, err := h.Port.CreateMilestone(ctx, Owner, Repo, forgejo_sdk.CreateMilestoneOption{Title: "set 1", Description: "first", Deadline: &due})
			must(t, err)
			if created.ID == 0 || created.Title != "set 1" || created.Description != "first" || created.State != forgejo_sdk.StateOpen || created.Deadline == nil {
				t.Fatalf("created: %+v", created)
			}

			desc := "fixed"
			edited, err := h.Port.EditMilestone(ctx, Owner, Repo, created.ID, forgejo_sdk.EditMilestoneOption{Description: &desc})
			must(t, err)
			if edited.Title != "set 1" || edited.Description != "fixed" || edited.Deadline == nil {
				t.Fatalf("partial edit must keep title and due date: %+v", edited)
			}

			closed := forgejo_sdk.StateClosed
			edited, err = h.Port.EditMilestone(ctx, Owner, Repo, created.ID, forgejo_sdk.EditMilestoneOption{State: &closed})
			must(t, err)
			if edited.State != forgejo_sdk.StateClosed || edited.Description != "fixed" {
				t.Fatalf("close: %+v", edited)
			}
			got, err := h.Port.GetMilestone(ctx, Owner, Repo, created.ID)
			must(t, err)
			if got.State != forgejo_sdk.StateClosed || got.Title != "set 1" {
				t.Fatalf("get after close: %+v", got)
			}

			open := forgejo_sdk.StateOpen
			_, err = h.Port.EditMilestone(ctx, Owner, Repo, created.ID, forgejo_sdk.EditMilestoneOption{State: &open})
			must(t, err)
			got, err = h.Port.GetMilestone(ctx, Owner, Repo, created.ID)
			must(t, err)
			if got.State != forgejo_sdk.StateOpen {
				t.Fatalf("get after reopen: %+v", got)
			}

			_, err = h.Port.GetMilestone(ctx, Owner, Repo, 999)
			wantNotFound(t, err)
			_, err = h.Port.EditMilestone(ctx, Owner, Repo, 999, forgejo_sdk.EditMilestoneOption{Description: &desc})
			wantNotFound(t, err)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.run(t, newHarness(t))
		})
	}
}

// newIssue creates an issue in Owner/Repo, making the repository first.
func newIssue(t *testing.T, h Harness, title string) *forgejo_sdk.Issue {
	t.Helper()
	h.Seed.AddRepo(Owner, Repo)
	iss, err := h.Port.CreateIssue(ctx, Owner, Repo, forgejo_sdk.CreateIssueOption{Title: title, Body: "body of " + title})
	must(t, err)
	return iss
}

func closeIssue(t *testing.T, h Harness, index int64) {
	t.Helper()
	closed := forgejo_sdk.StateClosed
	_, err := h.Port.EditIssue(ctx, Owner, Repo, index, forgejo_sdk.EditIssueOption{State: &closed})
	must(t, err)
}

func wantIssueIndices(t *testing.T, got []*forgejo_sdk.Issue, want ...int64) {
	t.Helper()
	have := make([]int64, len(got))
	for i, iss := range got {
		have[i] = iss.Index
	}
	if fmt.Sprint(have) != fmt.Sprint(append([]int64{}, want...)) {
		t.Fatalf("got indices %v, want %v", have, want)
	}
}

func wantLabels(t *testing.T, h Harness, index int64, want ...string) {
	t.Helper()
	got, err := h.Port.GetIssue(ctx, Owner, Repo, index)
	must(t, err)
	have := make([]string, len(got.Labels))
	for i, l := range got.Labels {
		have[i] = l.Name
	}
	if len(have) == 0 && len(want) == 0 {
		return
	}
	if fmt.Sprint(have) != fmt.Sprint(want) {
		t.Fatalf("labels = %v, want %v", have, want)
	}
}
