// SPDX-License-Identifier: GPL-3.0-or-later

package pull_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	apppull "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/pull"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/fake"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/pulladapter"
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
	if err != nil || len(got) != 2 {
		t.Fatalf("default list: %v, %v", got, err)
	}
	got, _ = s.List(context.Background(), apppull.ListInput{Owner: owner, Repo: repo, Limit: 1})
	if len(got) != 1 || got[0].Title != "open two" {
		t.Fatalf("limit 1 should be the newest: %v", got)
	}
	got, _ = s.List(context.Background(), apppull.ListInput{Owner: owner, Repo: repo, State: "all"})
	if len(got) != 3 {
		t.Fatalf("state=all: %v", got)
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

const diff = "diff --git a/a.go b/a.go\n+a\ndiff --git a/b.go b/b.go\n+b\n"

func TestDiffWholeOrOneFile(t *testing.T) {
	t.Parallel()
	s, f := newService(t)
	i := open(t, s, "diffed")
	if err := f.SetPullRequestDiff(owner, repo, i, diff); err != nil {
		t.Fatal(err)
	}
	got, err := s.Diff(context.Background(), apppull.DiffInput{PR: pr(i)})
	if err != nil || got != diff {
		t.Fatalf("whole: %q, %v", got, err)
	}
	got, err = s.Diff(context.Background(), apppull.DiffInput{PR: pr(i), FilePath: "b.go"})
	if err != nil || strings.Contains(got, "a.go") || !strings.Contains(got, "b.go") {
		t.Fatalf("slice: %q, %v", got, err)
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
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v, %v", list, err)
	}
	if err := s.DeleteReview(ctx, apppull.ReviewRef{PR: pr(i), ID: rev.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Review(ctx, apppull.ReviewRef{PR: pr(i), ID: rev.ID}); err == nil {
		t.Fatal("a deleted review must be gone")
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
	comments, err := s.ReviewComments(ctx, apppull.ReviewRef{PR: pr(i), ID: rev.ID})
	if err != nil || len(comments) != 1 || comments[0].Body != "why?" || comments[0].Path != "a.go" {
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
	if err != nil || len(reqs) != 2 {
		t.Fatalf("want two requested reviews, got %v, %v", reqs, err)
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
