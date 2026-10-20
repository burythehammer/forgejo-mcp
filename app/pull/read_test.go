// SPDX-License-Identifier: GPL-3.0-or-later

package pull_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/model"
	apppull "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/pull"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge/pulladapter"
)

// failingThread is a port whose pull thread listing fails, so a read that asks
// for comments has to report the failure rather than an empty thread.
type failingThread struct{ apppull.Port }

func (failingThread) ListPullRequestComments(context.Context, string, string, int64, apppull.Paging) ([]*model.Comment, error) {
	return nil, errors.New("comments boom")
}

func TestReadAloneReturnsOnlyThePullRequest(t *testing.T) {
	t.Parallel()
	s, f := newService(t)
	i := open(t, s, "feat: alone")
	if _, err := f.AddPullRequestComment(owner, repo, i, "bob", "hi"); err != nil {
		t.Fatal(err)
	}
	read, err := s.Read(context.Background(), pr(i), apppull.ReadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if read.PR == nil || read.PR.Title != "feat: alone" || read.PR.Index != i {
		t.Fatalf("PR = %+v", read.PR)
	}
	if read.Comments != nil || read.CommentsTruncated || read.Reviews != nil || read.ReviewsTruncated {
		t.Fatalf("zero options must read the PR alone, got %+v", read)
	}
}

func TestReadCapsTheThreadAndSaysSo(t *testing.T) {
	t.Parallel()
	s, f := newService(t)
	i := open(t, s, "chatty")
	for _, body := range []string{"one", "two", "three"} {
		if _, err := f.AddPullRequestComment(owner, repo, i, "bob", body); err != nil {
			t.Fatal(err)
		}
	}

	under, err := s.Read(context.Background(), pr(i), apppull.ReadOptions{CommentCap: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(under.Comments) != 3 || under.CommentsTruncated {
		t.Fatalf("at the cap: got %d comments, truncated=%v; want 3, false", len(under.Comments), under.CommentsTruncated)
	}

	over, err := s.Read(context.Background(), pr(i), apppull.ReadOptions{CommentCap: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(over.Comments) != 2 || !over.CommentsTruncated {
		t.Fatalf("over the cap: got %d comments, truncated=%v; want 2, true", len(over.Comments), over.CommentsTruncated)
	}
	if over.Comments[0].Body != "one" || over.Comments[1].Body != "two" {
		t.Fatalf("the cap keeps the first comments in Forgejo's order, got %q, %q", over.Comments[0].Body, over.Comments[1].Body)
	}
}

func TestReadCapsTheReviewsAndSaysSo(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	i := open(t, s, "reviewed")
	for range 3 {
		if _, err := s.CreateReview(context.Background(), apppull.CreateReviewRequest{PR: pr(i), State: "COMMENT", Body: "r"}); err != nil {
			t.Fatal(err)
		}
	}
	over, err := s.Read(context.Background(), pr(i), apppull.ReadOptions{ReviewCap: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(over.Reviews) != 2 || !over.ReviewsTruncated {
		t.Fatalf("got %d reviews, truncated=%v; want 2, true", len(over.Reviews), over.ReviewsTruncated)
	}
	if over.Comments != nil {
		t.Fatalf("a review-only read must not read the thread, got %d comments", len(over.Comments))
	}
}

func TestReadUnknownIsWrapped(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	_, err := s.Read(context.Background(), pr(99), apppull.ReadOptions{})
	if err == nil || !strings.HasPrefix(err.Error(), "get pull request err: ") {
		t.Fatalf("got %v", err)
	}
}

func TestReadThreadFailureIsAnError(t *testing.T) {
	t.Parallel()
	f := fake.New()
	f.AddRepo(owner, repo)
	s := apppull.New(failingThread{pulladapter.New(f)})
	i := open(t, s, "thread fails")
	_, err := s.Read(context.Background(), pr(i), apppull.ReadOptions{CommentCap: 30})
	if err == nil || !strings.HasPrefix(err.Error(), "list pull request comments err: ") {
		t.Fatalf("a failed thread listing must be reported, got %v", err)
	}
}
