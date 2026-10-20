// SPDX-License-Identifier: GPL-3.0-or-later

package issue_test

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"

	apppissue "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/issue"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/model"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge/issueadapter"
)

// These tests run the use cases over the in-memory fake, through the port
// bridge, so they exercise what the MCP adapters exercise without MCP in the
// way. Each test builds its own fake and runs in parallel with the rest.

const (
	owner = "octo"
	repo  = "hello"
)

func fixture(t *testing.T) (*apppissue.Service, *fake.Forgejo) {
	t.Helper()
	f := fake.New()
	f.AddRepo(owner, repo)
	return apppissue.New(issueadapter.New(f)), f
}

func target(index int64) apppissue.Target {
	return apppissue.Target{Owner: owner, Repo: repo, Index: index}
}

func names(labels []*model.Label) string {
	out := make([]string, 0, len(labels))
	for _, l := range labels {
		out = append(out, l.Name)
	}
	return strings.Join(out, ",")
}

func mustCreate(t *testing.T, s *apppissue.Service, in apppissue.CreateInput) *model.Issue {
	t.Helper()
	iss, err := s.Create(context.Background(), in)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	return iss
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func str(s string) *string { return &s }

func TestCreateIssue_LabelNamesAndIDsResolve(t *testing.T) {
	t.Parallel()
	s, f := fixture(t)
	bug := f.SeedIssueLabel(owner, repo, "bug")
	ui := f.SeedIssueLabel(owner, repo, "ui")

	iss := mustCreate(t, s, apppissue.CreateInput{Owner: owner, Repo: repo, Title: "t", Labels: str(fmt.Sprintf("bug,%d", ui))})
	if got := names(iss.Labels); got != "bug,ui" {
		t.Fatalf("labels = %q, want bug,ui (ids %d and %d)", got, bug, ui)
	}
}

func TestCreateIssue_DuplicateLabelsCollapse(t *testing.T) {
	t.Parallel()
	s, f := fixture(t)
	f.SeedIssueLabel(owner, repo, "bug")

	iss := mustCreate(t, s, apppissue.CreateInput{Owner: owner, Repo: repo, Title: "t", Labels: str("bug, bug ,bug")})
	if got := names(iss.Labels); got != "bug" {
		t.Fatalf("labels = %q, want one bug", got)
	}
}

func TestCreateIssue_NameBeatsID(t *testing.T) {
	t.Parallel()
	s, f := fixture(t)
	f.SeedIssueLabel(owner, repo, "x")
	f.SeedIssueLabel(owner, repo, "y") // ID 2 belongs to "y"
	f.SeedIssueLabel(owner, repo, "2") // a label literally named "2"

	iss := mustCreate(t, s, apppissue.CreateInput{Owner: owner, Repo: repo, Title: "t", Labels: str("2")})
	if got := names(iss.Labels); got != "2" {
		t.Fatalf("labels = %q, want the label named 2", got)
	}
}

func TestCreateIssue_UnresolvedLabelsWriteNothing(t *testing.T) {
	t.Parallel()
	s, f := fixture(t)
	f.SeedIssueLabel(owner, repo, "bug")

	cases := []struct{ labels, want string }{
		{"bug,nope", `unknown label "nope"`},
		{"", "labels names nothing"},
		{" , ", "labels names nothing"},
		{"99", `label "99" is neither a label name nor the ID`},
	}
	for _, c := range cases {
		_, err := s.Create(context.Background(), apppissue.CreateInput{Owner: owner, Repo: repo, Title: "t", Labels: str(c.labels)})
		if !strings.Contains(errText(err), c.want) {
			t.Errorf("labels %q: error %q, want it to contain %q", c.labels, errText(err), c.want)
		}
	}
	page, err := s.List(context.Background(), apppissue.ListInput{Owner: owner, Repo: repo, State: "all", Paging: apppissue.Paging{Page: 1, Limit: 20}})
	if err != nil || len(page.Issues) != 0 {
		t.Fatalf("issues after failed creates = %d (err %v), want none", len(page.Issues), err)
	}
}

func TestResolveLabels_AmbiguousAcrossScopes(t *testing.T) {
	t.Parallel()
	f := fake.New()
	f.AddRepo("acme", repo)
	s := apppissue.New(issueadapter.New(f))
	ctx := context.Background()
	f.SeedIssueLabel("acme", repo, "ui")
	if _, err := s.CreateOrgLabel(ctx, "acme", apppissue.LabelNew{Name: "ui", Color: "#00aaaa"}); err != nil {
		t.Fatal(err)
	}

	_, err := s.Create(ctx, apppissue.CreateInput{Owner: "acme", Repo: repo, Title: "t", Labels: str("ui")})
	want := `label "ui" is ambiguous: it matches organization label`
	if !strings.Contains(errText(err), want) || !strings.Contains(errText(err), "repository label") {
		t.Fatalf("error = %q, want an ambiguity naming both scopes (%s...)", errText(err), want)
	}
}

// The catalogue is paged 100 at a time. A label on a later page must still
// resolve, or it would be reported as unknown.
func TestResolveLabels_LabelOnALaterPage(t *testing.T) {
	t.Parallel()
	s, f := fixture(t)
	for i := 0; i < 150; i++ {
		f.SeedIssueLabel(owner, repo, "filler-"+strconv.Itoa(i))
	}
	f.SeedIssueLabel(owner, repo, "late")

	iss := mustCreate(t, s, apppissue.CreateInput{Owner: owner, Repo: repo, Title: "t", Labels: str("late")})
	if got := names(iss.Labels); got != "late" {
		t.Fatalf("labels = %q, want late", got)
	}
}

func TestUpdateIssue_SetLabelsReplacesThenRereads(t *testing.T) {
	t.Parallel()
	s, f := fixture(t)
	f.SeedIssueLabel(owner, repo, "bug")
	f.SeedIssueLabel(owner, repo, "ui")
	ctx := context.Background()
	iss := mustCreate(t, s, apppissue.CreateInput{Owner: owner, Repo: repo, Title: "t", Labels: str("bug")})

	got, err := s.Update(ctx, apppissue.UpdateInput{Target: target(iss.Index), Title: "renamed", SetLabels: str("ui")})
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "renamed" || names(got.Labels) != "ui" {
		t.Fatalf("after update: title %q labels %q, want renamed / ui", got.Title, names(got.Labels))
	}
}

func TestUpdateIssue_EmptySetLabelsClears(t *testing.T) {
	t.Parallel()
	s, f := fixture(t)
	f.SeedIssueLabel(owner, repo, "bug")
	iss := mustCreate(t, s, apppissue.CreateInput{Owner: owner, Repo: repo, Title: "t", Labels: str("bug")})

	got, err := s.Update(context.Background(), apppissue.UpdateInput{Target: target(iss.Index), SetLabels: str("")})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Labels) != 0 {
		t.Fatalf("labels after clear = %q", names(got.Labels))
	}
}

func TestUpdateIssue_UnknownLabelLeavesTheIssueUntouched(t *testing.T) {
	t.Parallel()
	s, _ := fixture(t)
	ctx := context.Background()
	iss := mustCreate(t, s, apppissue.CreateInput{Owner: owner, Repo: repo, Title: "original"})

	_, err := s.Update(ctx, apppissue.UpdateInput{Target: target(iss.Index), Title: "changed", SetLabels: str("nope")})
	if !strings.Contains(errText(err), `unknown label "nope"`) {
		t.Fatalf("error = %q", errText(err))
	}
	read, err := s.Read(ctx, target(iss.Index), apppissue.ReadOptions{})
	if err != nil || read.Issue.Title != "original" {
		t.Fatalf("read = %+v (err %v), want the title unchanged", read.Issue, err)
	}
}

func TestUpdateIssue_ArgumentErrorsComeBeforeAnyWrite(t *testing.T) {
	t.Parallel()
	s, _ := fixture(t)
	ctx := context.Background()
	iss := mustCreate(t, s, apppissue.CreateInput{Owner: owner, Repo: repo, Title: "original"})

	cases := []struct {
		in   apppissue.UpdateInput
		want string
	}{
		{apppissue.UpdateInput{Milestone: "x"}, "invalid milestone ID"},
		{apppissue.UpdateInput{DueDate: "soon"}, "invalid due_date format (expected RFC3339)"},
		{apppissue.UpdateInput{DueDate: "2026-08-20T00:00:00Z", ClearDueDate: true}, "cannot set 'due_date' and 'clear_due_date' at the same time"},
	}
	for _, c := range cases {
		c.in.Target = target(iss.Index)
		c.in.Title = "changed"
		if _, err := s.Update(ctx, c.in); !strings.Contains(errText(err), c.want) {
			t.Errorf("error = %q, want it to contain %q", errText(err), c.want)
		}
	}
	if read, _ := s.Read(ctx, target(iss.Index), apppissue.ReadOptions{}); read.Issue.Title != "original" {
		t.Fatalf("title = %q, want the original", read.Issue.Title)
	}
}

func TestUpdateIssue_AssigneesEmptyListClears(t *testing.T) {
	t.Parallel()
	s, _ := fixture(t)
	ctx := context.Background()
	iss := mustCreate(t, s, apppissue.CreateInput{Owner: owner, Repo: repo, Title: "t", Assignees: str("alice")})
	if len(iss.Assignees) != 1 {
		t.Fatalf("assignees on create = %d, want 1", len(iss.Assignees))
	}
	got, err := s.Update(ctx, apppissue.UpdateInput{Target: target(iss.Index), Assignees: str("")})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Assignees) != 0 {
		t.Fatalf("assignees after clear = %d", len(got.Assignees))
	}
}

func TestChangeState_RejectsOtherStates(t *testing.T) {
	t.Parallel()
	s, _ := fixture(t)
	iss := mustCreate(t, s, apppissue.CreateInput{Owner: owner, Repo: repo, Title: "t"})
	_, err := s.ChangeState(context.Background(), target(iss.Index), "merged")
	if got, want := errText(err), "invalid state: merged, must be 'open' or 'closed'"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestList_ArgumentValidation(t *testing.T) {
	t.Parallel()
	s, _ := fixture(t)
	ctx := context.Background()
	cases := []struct {
		in   apppissue.ListInput
		want string
	}{
		{apppissue.ListInput{Repo: repo}, "owner is required"},
		{apppissue.ListInput{Owner: owner}, "repo is required; for owner-wide listing across repositories use search_issues instead"},
		{apppissue.ListInput{Owner: owner, Repo: repo, Sort: "random"}, `invalid sort "random": must be one of relevance, latest,`},
	}
	for _, c := range cases {
		if _, err := s.List(ctx, c.in); !strings.Contains(errText(err), c.want) {
			t.Errorf("error = %q, want it to contain %q", errText(err), c.want)
		}
	}
}

func TestSearch_EnvelopeAndCeiling(t *testing.T) {
	t.Parallel()
	s, f := fixture(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		mustCreate(t, s, apppissue.CreateInput{Owner: owner, Repo: repo, Title: "t" + strconv.Itoa(i)})
	}
	f.SetMaxResponseItems(10)

	page, err := s.Search(ctx, apppissue.SearchInput{Owner: owner, Paging: apppissue.Paging{Page: 1, Limit: 2}})
	if err != nil {
		t.Fatal(err)
	}
	if page.Count != 2 || !page.HasNext || page.Limit != 2 || page.Page != 1 {
		t.Fatalf("page = %+v, want two issues with a next page", page)
	}

	_, err = s.Search(ctx, apppissue.SearchInput{Owner: owner, Paging: apppissue.Paging{Page: 1, Limit: 11}})
	if want := "limit 11 exceeds this instance's maximum of 10"; !strings.Contains(errText(err), want) {
		t.Fatalf("error = %q, want %q", errText(err), want)
	}
}

func TestSearch_OwnerRequired(t *testing.T) {
	t.Parallel()
	s, _ := fixture(t)
	if _, err := s.Search(context.Background(), apppissue.SearchInput{}); errText(err) != "owner is required" {
		t.Fatalf("error = %q", errText(err))
	}
}

func TestComments_TimeWindowMustBeRFC3339(t *testing.T) {
	t.Parallel()
	s, _ := fixture(t)
	iss := mustCreate(t, s, apppissue.CreateInput{Owner: owner, Repo: repo, Title: "t"})
	_, err := s.Comments(context.Background(), apppissue.CommentsInput{Target: target(iss.Index), Since: "yesterday"})
	if !strings.HasPrefix(errText(err), "invalid since time format (expected RFC3339)") {
		t.Fatalf("error = %q", errText(err))
	}
}

func TestRepoLabel_NormalisesColourAndRejectsBadOnes(t *testing.T) {
	t.Parallel()
	s, _ := fixture(t)
	ctx := context.Background()

	l, err := s.CreateRepoLabel(ctx, owner, repo, apppissue.LabelNew{Name: "a", Color: "#0088FF"})
	if err != nil {
		t.Fatal(err)
	}
	// The fake stores colours without their leading #, as Forgejo does; the
	// lower-casing is what this checks.
	if l.Color != "0088ff" {
		t.Fatalf("color = %q, want 0088ff", l.Color)
	}
	_, err = s.CreateRepoLabel(ctx, owner, repo, apppissue.LabelNew{Name: "b", Color: "#abc"})
	if !strings.Contains(errText(err), "color must be a 6-digit hex string") {
		t.Fatalf("error = %q", errText(err))
	}
}

func TestRepoLabel_EditNeedsAField(t *testing.T) {
	t.Parallel()
	s, f := fixture(t)
	id := f.SeedIssueLabel(owner, repo, "a")
	_, err := s.EditRepoLabel(context.Background(), owner, repo, id, apppissue.LabelEdit{Tool: "edit_repo_label"})
	if want := "edit_repo_label: at least one of name, color, description must be provided"; errText(err) != want {
		t.Fatalf("error = %q, want %q", errText(err), want)
	}
}

func TestRepoLabel_EmptyNameIsLeftAlone(t *testing.T) {
	t.Parallel()
	s, f := fixture(t)
	id := f.SeedIssueLabel(owner, repo, "keep")
	got, err := s.EditRepoLabel(context.Background(), owner, repo, id, apppissue.LabelEdit{Tool: "edit_repo_label", Name: str(""), Description: str("d")})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "keep" || got.Description != "d" {
		t.Fatalf("label = %+v, want the name kept and the description set", got)
	}
}

func TestDeleteRepoLabel_SafeRefusesWhileInUse(t *testing.T) {
	t.Parallel()
	s, f := fixture(t)
	ctx := context.Background()
	id := f.SeedIssueLabel(owner, repo, "bug")
	mustCreate(t, s, apppissue.CreateInput{Owner: owner, Repo: repo, Title: "t", Labels: str("bug")})

	err := s.DeleteRepoLabel(ctx, owner, repo, id, false)
	if !strings.Contains(errText(err), `label "bug" is used by 1 issue(s)/PR(s); set delete_mode=force`) {
		t.Fatalf("error = %q", errText(err))
	}
	if err := s.DeleteRepoLabel(ctx, owner, repo, id, true); err != nil {
		t.Fatalf("forced delete: %v", err)
	}
}

func TestDeleteRepoLabel_MissingIsNotFound(t *testing.T) {
	t.Parallel()
	s, _ := fixture(t)
	err := s.DeleteRepoLabel(context.Background(), owner, repo, 777, true)
	if got := errText(err); got != "label 777 not found" {
		t.Fatalf("error = %q, want label 777 not found", got)
	}
}

func TestDeleteOrgLabel_RefusedWhileAnOrgRepoUsesIt(t *testing.T) {
	t.Parallel()
	f := fake.New()
	f.AddRepo("acme", repo)
	s := apppissue.New(issueadapter.New(f))
	ctx := context.Background()
	l, err := s.CreateOrgLabel(ctx, "acme", apppissue.LabelNew{Name: "infra", Color: "#0000ff"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(ctx, apppissue.CreateInput{Owner: "acme", Repo: repo, Title: "t", Labels: str("infra")}); err != nil {
		t.Fatal(err)
	}

	err = s.DeleteOrgLabel(ctx, "acme", l.ID, false)
	if !strings.Contains(errText(err), `label "infra" is used by 1 issue(s)/PR(s) in visible repos`) {
		t.Fatalf("error = %q", errText(err))
	}
	if err := s.DeleteOrgLabel(ctx, "acme", l.ID, true); err != nil {
		t.Fatalf("forced delete: %v", err)
	}
}

func TestAddDependency_SelfIsRefusedWhateverTheCase(t *testing.T) {
	t.Parallel()
	s, _ := fixture(t)
	ctx := context.Background()
	iss := mustCreate(t, s, apppissue.CreateInput{Owner: owner, Repo: repo, Title: "t"})

	self := apppissue.Target{Owner: "OCTO", Repo: "HELLO", Index: iss.Index}
	if err := s.AddDependency(ctx, target(iss.Index), self); errText(err) != "an issue cannot depend on itself" {
		t.Fatalf("error = %q", errText(err))
	}
}

func TestAddDependency_SameIndexInAnotherRepoIsAllowed(t *testing.T) {
	t.Parallel()
	s, f := fixture(t)
	ctx := context.Background()
	f.AddRepo("acme", "other")
	iss := mustCreate(t, s, apppissue.CreateInput{Owner: owner, Repo: repo, Title: "t"})
	other := mustCreate(t, s, apppissue.CreateInput{Owner: "acme", Repo: "other", Title: "o"})

	dep := apppissue.Target{Owner: "acme", Repo: "other", Index: other.Index}
	if err := s.AddDependency(ctx, target(iss.Index), dep); err != nil {
		t.Fatalf("add: %v", err)
	}
	page, err := s.Dependencies(ctx, target(iss.Index), apppissue.Paging{Page: 1, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if page.Page != 1 || page.Limit != 20 {
		t.Fatalf("page echo = %d/%d, want 1/20", page.Page, page.Limit)
	}
}

// normalizeColor is observed through CreateRepoLabel: the colour Forgejo
// stores is the normalised one, without its #. A rejected colour is never sent.
func TestNormalizeColor(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in, out string
		bad     bool
	}{
		{"0088ff", "0088ff", false},
		{"#0088ff", "0088ff", false},
		{"#0088FF", "0088ff", false},
		{"0088FF", "0088ff", false},
		{"abc", "", true},    // 3-digit rejected
		{"#abc", "", true},   // 3-digit rejected
		{"gggggg", "", true}, // invalid hex
		{"", "", true},
	}
	for i, c := range cases {
		s, _ := fixture(t)
		l, err := s.CreateRepoLabel(context.Background(), owner, repo, apppissue.LabelNew{Name: fmt.Sprintf("l%d", i), Color: c.in})
		switch {
		case c.bad && err == nil:
			t.Errorf("colour %q: want an error, got %q", c.in, l.Color)
		case !c.bad && err != nil:
			t.Errorf("colour %q: unexpected error: %v", c.in, err)
		case !c.bad && l.Color != c.out:
			t.Errorf("colour %q: stored %q, want %q", c.in, l.Color, c.out)
		}
	}
}

// Forgejo labels a pull request through the same issue endpoints, and the
// resolver never reads the index, so a name on a pull request's index is refused
// by the same rule, before any write.
func TestAddLabels_PullRequestIndexResolvesByTheIssueRule(t *testing.T) {
	t.Parallel()
	s, f := fixture(t)
	f.SeedIssueLabel(owner, repo, "bug")
	ctx := context.Background()

	if _, err := s.AddLabels(ctx, target(7), "nope"); !strings.Contains(errText(err), `unknown label "nope"`) {
		t.Fatalf("unknown name on a pull request: error %q, want the shared unknown-label error", errText(err))
	}
	if _, err := s.RemoveLabels(ctx, target(7), " , "); !strings.Contains(errText(err), "labels names nothing") {
		t.Fatalf("empty list on a pull request: error %q, want the shared names-nothing error", errText(err))
	}
}

func TestList_EnvelopeNamesTheNextPage(t *testing.T) {
	t.Parallel()
	s, f := fixture(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		mustCreate(t, s, apppissue.CreateInput{Owner: owner, Repo: repo, Title: "t" + strconv.Itoa(i)})
	}
	f.SetMaxResponseItems(10)

	first, err := s.List(ctx, apppissue.ListInput{Owner: owner, Repo: repo, State: "all", Paging: apppissue.Paging{Page: 1, Limit: 2}})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Issues) != 2 || first.Count != 2 || first.Page != 1 || first.Limit != 2 || !first.HasNext {
		t.Fatalf("first page = %+v, want two issues with a next page", first)
	}

	last, err := s.List(ctx, apppissue.ListInput{Owner: owner, Repo: repo, State: "all", Paging: apppissue.Paging{Page: 2, Limit: 2}})
	if err != nil {
		t.Fatal(err)
	}
	if len(last.Issues) != 1 || last.Count != 1 || last.HasNext {
		t.Fatalf("last page = %+v, want one issue and no next page", last)
	}
}
