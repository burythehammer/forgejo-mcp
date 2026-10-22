// SPDX-License-Identifier: GPL-3.0-or-later

package pull_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	apppull "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/pull"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/pulladapter"
)

// The use cases run against the in-memory fake through the pull adapter: no
// MCP, no HTTP. The fake is built per test, so tests run in parallel.

const (
	owner = "octo"
	repo  = "hello"
)

func newService(t *testing.T) (*apppull.Service, *fake.Forgejo) {
	t.Helper()
	f := fake.New()
	f.AddRepo(owner, repo)
	return apppull.New(pulladapter.New(f)), f
}

func open(t *testing.T, s *apppull.Service, title string) int64 {
	t.Helper()
	pr, err := s.Create(context.Background(), apppull.CreateRequest{Owner: owner, Repo: repo, Head: "feature", Base: "main", Title: title, Body: "b"})
	if err != nil {
		t.Fatal(err)
	}
	return pr.Index
}

func pr(i int64) apppull.PR { return apppull.PR{Owner: owner, Repo: repo, Index: i} }

func TestReadReturnsWhatWasCreated(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	i := open(t, s, "feat: x")
	read, err := s.Read(context.Background(), pr(i), apppull.ReadOptions{})
	if err != nil || read.PR.Title != "feat: x" || read.PR.Index != i {
		t.Fatalf("got %+v, %v", read.PR, err)
	}
}

func TestListDefaultsToOpenAndPaginates(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	merged := open(t, s, "already merged")
	open(t, s, "open one")
	open(t, s, "open two")
	if _, err := s.Merge(context.Background(), apppull.MergeRequest{PR: pr(merged), Style: "merge"}); err != nil {
		t.Fatal(err)
	}

	got, err := s.List(context.Background(), apppull.ListInput{Owner: owner, Repo: repo})
	if err != nil || len(got.PullRequests) != 2 {
		t.Fatalf("default list: %+v, %v", got, err)
	}
	got, _ = s.List(context.Background(), apppull.ListInput{Owner: owner, Repo: repo, Limit: 1})
	if len(got.PullRequests) != 1 || got.PullRequests[0].Title != "open two" {
		t.Fatalf("limit 1 should be the newest: %+v", got)
	}
	got, _ = s.List(context.Background(), apppull.ListInput{Owner: owner, Repo: repo, State: "all"})
	if len(got.PullRequests) != 3 {
		t.Fatalf("state=all: %+v", got)
	}
}

func TestListEnvelopeNamesTheNextPage(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	ctx := context.Background()
	for _, title := range []string{"one", "two", "three", "four"} {
		open(t, s, title)
	}
	first, err := s.List(ctx, apppull.ListInput{Owner: owner, Repo: repo, Page: 1, Limit: 2})
	if err != nil || len(first.PullRequests) != 2 || first.Count != 2 || first.Page != 1 || first.Limit != 2 || !first.HasNext {
		t.Fatalf("page 1 of four: %+v, %v", first, err)
	}
	// Four rows at limit 2: page 2 is full and is the last, so the probe
	// finds nothing and has_next is false.
	last, err := s.List(ctx, apppull.ListInput{Owner: owner, Repo: repo, Page: 2, Limit: 2})
	if err != nil || last.Count != 2 || last.HasNext {
		t.Fatalf("exact-multiple last page: %+v, %v", last, err)
	}
	past, err := s.List(ctx, apppull.ListInput{Owner: owner, Repo: repo, Page: 3, Limit: 2})
	if err != nil || past.Count != 0 || past.HasNext {
		t.Fatalf("past the end: %+v, %v", past, err)
	}
}

func TestInvalidMilestoneIsInvalidInput(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	_, err := s.List(context.Background(), apppull.ListInput{Owner: owner, Repo: repo, Milestone: "soon"})
	var bad *apppull.InvalidInputError
	if !errors.As(err, &bad) || !strings.HasPrefix(err.Error(), "invalid milestone ID: ") {
		t.Fatalf("got %v", err)
	}
	_, err = s.Update(context.Background(), apppull.UpdateInput{PR: pr(1), Milestone: "x1"})
	if !errors.As(err, &bad) {
		t.Fatalf("update: got %v", err)
	}
}

func TestUpdateChangesOnlyWhatWasGiven(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	i := open(t, s, "first")
	if _, err := s.Update(context.Background(), apppull.UpdateInput{PR: pr(i), Title: "second"}); err != nil {
		t.Fatal(err)
	}
	read, _ := s.Read(context.Background(), pr(i), apppull.ReadOptions{})
	got := read.PR
	if got.Title != "second" || got.Body != "b" {
		t.Fatalf("got title %q body %q", got.Title, got.Body)
	}
}

func TestMergeReportsSchedulingAndDeclines(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	i := open(t, s, "to merge")
	res, err := s.Merge(context.Background(), apppull.MergeRequest{PR: pr(i), Style: "merge"})
	if err != nil || res.Scheduled {
		t.Fatalf("merge: %+v, %v", res, err)
	}
	// Forgejo declines a second merge; the status is reported.
	_, err = s.Merge(context.Background(), apppull.MergeRequest{PR: pr(i), Style: "merge"})
	var declined *apppull.MergeDeclinedError
	if !errors.As(err, &declined) || declined.StatusCode != 405 {
		t.Fatalf("got %v", err)
	}
	j := open(t, s, "scheduled")
	res, err = s.Merge(context.Background(), apppull.MergeRequest{PR: pr(j), Style: "merge", WhenChecksSucceed: true})
	if err != nil || !res.Scheduled {
		t.Fatalf("scheduled: %+v, %v", res, err)
	}
}

func TestMergeDeclinedWhenChecksSucceedReportsMessage(t *testing.T) {
	t.Parallel()
	s, f := newService(t)
	i := open(t, s, "declined but scheduled")
	if err := f.SetMergeDecline(owner, repo, i, 422, "Pull request has merge conflicts"); err != nil {
		t.Fatal(err)
	}
	res, err := s.Merge(context.Background(), apppull.MergeRequest{PR: pr(i), Style: "merge", WhenChecksSucceed: true})
	var declined *apppull.MergeDeclinedError
	if !errors.As(err, &declined) || declined.StatusCode != 422 || declined.Message != "Pull request has merge conflicts" {
		t.Fatalf("got %+v, %v; want MergeDeclinedError 422 with Forgejo's message", res, err)
	}
	if res.Scheduled {
		t.Fatal("a declined merge is not scheduled")
	}
}

const diff = "diff --git a/a.go b/a.go\n+a\ndiff --git a/b.go b/b.go\n+b\n"

func TestDiffWholeOrOneFile(t *testing.T) {
	t.Parallel()
	s, f := newService(t)
	i := open(t, s, "diffed")
	if err := f.SetPullRequestDiff(owner, repo, i, diff); err != nil {
		t.Fatal(err)
	}
	got, err := s.Diff(context.Background(), apppull.DiffInput{PR: pr(i)})
	if err != nil || got.Text != diff {
		t.Fatalf("whole: %+v, %v", got, err)
	}
	got, err = s.Diff(context.Background(), apppull.DiffInput{PR: pr(i), FilePath: "b.go"})
	if err != nil || strings.Contains(got.Text, "a.go") || !strings.Contains(got.Text, "b.go") {
		t.Fatalf("slice: %+v, %v", got, err)
	}
	_, err = s.Diff(context.Background(), apppull.DiffInput{PR: pr(i), FilePath: "nope.go"})
	var missing *apppull.DiffFileNotFoundError
	if !errors.As(err, &missing) || missing.Path != "nope.go" {
		t.Fatalf("got %v", err)
	}
}

func TestReviewLifecycle(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	ctx := context.Background()
	i := open(t, s, "reviewed")

	rev, err := s.CreateReview(ctx, apppull.CreateReviewRequest{PR: pr(i), State: "COMMENT", Body: "looks fine"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Review(ctx, apppull.ReviewRef{PR: pr(i), ID: rev.ID})
	if err != nil || got.Body != "looks fine" {
		t.Fatalf("got %+v, %v", got, err)
	}
	list, err := s.Reviews(ctx, apppull.PagedPR{PR: pr(i)})
	if err != nil || len(list.Reviews) != 1 {
		t.Fatalf("list: %+v, %v", list, err)
	}
	if err := s.DeleteReview(ctx, apppull.ReviewRef{PR: pr(i), ID: rev.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Review(ctx, apppull.ReviewRef{PR: pr(i), ID: rev.ID}); err == nil {
		t.Fatal("a deleted review must be gone")
	}
}

func TestReviewsEnvelopeNamesTheNextPage(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	ctx := context.Background()
	i := open(t, s, "reviewed")
	for range 3 {
		if _, err := s.CreateReview(ctx, apppull.CreateReviewRequest{PR: pr(i), State: "COMMENT", Body: "c"}); err != nil {
			t.Fatal(err)
		}
	}
	first, err := s.Reviews(ctx, apppull.PagedPR{PR: pr(i), Page: 1, Limit: 2})
	if err != nil || first.Count != 2 || first.Limit != 2 || !first.HasNext {
		t.Fatalf("page 1 of three: %+v, %v", first, err)
	}
	second, err := s.Reviews(ctx, apppull.PagedPR{PR: pr(i), Page: 2, Limit: 2})
	if err != nil || second.Count != 1 || second.HasNext {
		t.Fatalf("page 2 of three: %+v, %v", second, err)
	}
}

func TestInlineCommentsSubmitAndDismiss(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	ctx := context.Background()
	i := open(t, s, "x")

	rev, err := s.CreateReview(ctx, apppull.CreateReviewRequest{
		PR: pr(i), State: "PENDING", Body: "draft",
		Comments: `[{"path":"a.go","body":"why?","new_position":3}]`,
	})
	if err != nil {
		t.Fatal(err)
	}
	comments, err := s.ReviewComments(ctx, apppull.ReviewRef{PR: pr(i), ID: rev.ID}, apppull.Paging{})
	if err != nil || len(comments.Comments) != 1 || comments.Comments[0].Body != "why?" || comments.Comments[0].Path != "a.go" {
		t.Fatalf("comments: %+v, %v", comments, err)
	}
	sub, err := s.SubmitReview(ctx, apppull.SubmitReviewRequest{ReviewRef: apppull.ReviewRef{PR: pr(i), ID: rev.ID}, State: "APPROVED"})
	if err != nil || sub.State != "APPROVED" {
		t.Fatalf("submit: %+v, %v", sub, err)
	}
	if err := s.DismissReview(ctx, apppull.DismissRequest{ReviewRef: apppull.ReviewRef{PR: pr(i), ID: rev.ID}, Message: "stale"}); err != nil {
		t.Fatal(err)
	}
}

func TestCreateReviewRejectsBadCommentsJSON(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	i := open(t, s, "x")
	_, err := s.CreateReview(context.Background(), apppull.CreateReviewRequest{PR: pr(i), State: "COMMENT", Comments: "{"})
	var bad *apppull.InvalidInputError
	if !errors.As(err, &bad) || !strings.HasPrefix(err.Error(), "invalid comments JSON: ") {
		t.Fatalf("got %v", err)
	}
}

func TestReviewRequestsSplitAndTrimTheLists(t *testing.T) {
	t.Parallel()
	s, f := newService(t)
	ctx := context.Background()
	i := open(t, s, "x")
	f.AddUser("a")
	f.AddUser("b")
	if err := s.RequestReviews(ctx, apppull.ReviewersInput{PR: pr(i), Reviewers: "a, b,,"}); err != nil {
		t.Fatal(err)
	}
	reqs, err := s.Reviews(ctx, apppull.PagedPR{PR: pr(i)})
	if err != nil || len(reqs.Reviews) != 2 {
		t.Fatalf("want two requested reviews, got %+v, %v", reqs, err)
	}
	if err := s.CancelReviewRequests(ctx, apppull.ReviewersInput{PR: pr(i), Reviewers: "a"}); err != nil {
		t.Fatal(err)
	}
}

func TestPortFailureNamesTheOperation(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	err := s.DeleteReview(context.Background(), apppull.ReviewRef{PR: pr(99), ID: 1})
	if err == nil || !strings.HasPrefix(err.Error(), "delete pull review err: ") {
		t.Fatalf("got %v", err)
	}
}
