// SPDX-License-Identifier: GPL-3.0-or-later

package issue_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	apppissue "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/issue"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/issueadapter"
)

// commentsFail is a port whose comment listing fails. A read that asks for
// comments must report that failure, and a read that does not must never list
// comments at all.
type commentsFail struct{ apppissue.Port }

func (commentsFail) ListIssueComments(context.Context, string, string, int64, apppissue.CommentList) ([]*model.Comment, error) {
	return nil, errors.New("comments boom")
}

// seedReadIssue opens an issue with n comments through the use cases.
func seedReadIssue(t *testing.T, s *apppissue.Service, n int) int64 {
	t.Helper()
	ctx := context.Background()
	iss := mustCreate(t, s, apppissue.CreateInput{Owner: owner, Repo: repo, Title: "t", Body: "b"})
	for i := 0; i < n; i++ {
		if _, err := s.CreateComment(ctx, target(iss.Index), fmt.Sprint("c", i)); err != nil {
			t.Fatal(err)
		}
	}
	return iss.Index
}

func TestRead_IssueOnlyNeverListsComments(t *testing.T) {
	t.Parallel()
	s, f := fixture(t)
	idx := seedReadIssue(t, s, 2)
	s = apppissue.New(commentsFail{issueadapter.New(f)})

	read, err := s.Read(context.Background(), target(idx), apppissue.ReadOptions{})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if read.Issue == nil || read.Issue.Title != "t" {
		t.Fatalf("issue = %+v", read.Issue)
	}
	if read.Comments != nil || read.CommentsTruncated {
		t.Fatalf("comments = %v truncated = %v, want none", read.Comments, read.CommentsTruncated)
	}
}

func TestRead_RecentCommentsCapped(t *testing.T) {
	t.Parallel()
	s, _ := fixture(t)
	idx := seedReadIssue(t, s, 31)

	read, err := s.Read(context.Background(), target(idx), apppissue.ReadOptions{CommentCap: 30})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(read.Comments) != 30 || !read.CommentsTruncated {
		t.Fatalf("comments = %d truncated = %v, want 30 and truncated", len(read.Comments), read.CommentsTruncated)
	}
	if read.Comments[0].Body != "c0" {
		t.Fatalf("first comment = %q, want the oldest", read.Comments[0].Body)
	}
}

func TestRead_ExactlyCapIsNotTruncated(t *testing.T) {
	t.Parallel()
	s, _ := fixture(t)
	idx := seedReadIssue(t, s, 30)

	read, err := s.Read(context.Background(), target(idx), apppissue.ReadOptions{CommentCap: 30})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(read.Comments) != 30 || read.CommentsTruncated {
		t.Fatalf("comments = %d truncated = %v, want 30 and not truncated", len(read.Comments), read.CommentsTruncated)
	}
}

func TestRead_CommentFailureIsReported(t *testing.T) {
	t.Parallel()
	s, f := fixture(t)
	idx := seedReadIssue(t, s, 1)
	s = apppissue.New(commentsFail{issueadapter.New(f)})

	if _, err := s.Read(context.Background(), target(idx), apppissue.ReadOptions{CommentCap: 30}); err == nil {
		t.Fatal("want the comment failure, got nil")
	}
}

func TestRead_MissingIssueIsAnError(t *testing.T) {
	t.Parallel()
	s, _ := fixture(t)
	if _, err := s.Read(context.Background(), target(99), apppissue.ReadOptions{}); err == nil {
		t.Fatal("want an error for a missing issue")
	}
}

// pagingSpy records the comment listing the read asked for, and passes it on.
type pagingSpy struct {
	apppissue.Port
	asked *apppissue.CommentList
}

func (p pagingSpy) ListIssueComments(ctx context.Context, owner, repo string, index int64, c apppissue.CommentList) ([]*model.Comment, error) {
	*p.asked = c
	return p.Port.ListIssueComments(ctx, owner, repo, index, c)
}

// The read asks for one comment more than the cap, so "more than the cap" is
// visible without a second call. The limit must reach the port as cap+1.
func TestRead_AsksForOneMoreThanTheCap(t *testing.T) {
	t.Parallel()
	seeded, f := fixture(t)
	idx := seedReadIssue(t, seeded, 31)
	var asked apppissue.CommentList
	s := apppissue.New(pagingSpy{Port: issueadapter.New(f), asked: &asked})

	if _, err := s.Read(context.Background(), target(idx), apppissue.ReadOptions{CommentCap: 30}); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if asked.Limit != 31 {
		t.Fatalf("comment listing limit = %d, want cap+1 = 31", asked.Limit)
	}
}

// seedKinds opens an issue and a pull request, which share one index sequence,
// and returns their indexes.
func seedKinds(t *testing.T, s *apppissue.Service, f interface {
	MarkPullRequest(owner, repo string, index int64)
}) (issue, pull int64) {
	t.Helper()
	issue = mustCreate(t, s, apppissue.CreateInput{Owner: owner, Repo: repo, Title: "an issue"}).Index
	pull = mustCreate(t, s, apppissue.CreateInput{Owner: owner, Repo: repo, Title: "a pull request"}).Index
	f.MarkPullRequest(owner, repo, pull)
	return issue, pull
}

func TestRead_KindFilter(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cases := []struct {
		name    string
		kind    apppissue.Kind
		pull    bool
		wantErr bool
		want    apppissue.Kind
	}{
		{name: "any accepts an issue", kind: apppissue.KindAny, pull: false},
		{name: "any accepts a pull request", kind: apppissue.KindAny, pull: true},
		{name: "issues accepts an issue", kind: apppissue.KindIssues, pull: false},
		{name: "issues refuses a pull request", kind: apppissue.KindIssues, pull: true, wantErr: true, want: apppissue.KindIssues},
		{name: "pulls accepts a pull request", kind: apppissue.KindPulls, pull: true},
		{name: "pulls refuses an issue", kind: apppissue.KindPulls, pull: false, wantErr: true, want: apppissue.KindPulls},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s, f := fixture(t)
			issue, pull := seedKinds(t, s, f)
			idx := issue
			if c.pull {
				idx = pull
			}
			read, err := s.Read(ctx, target(idx), apppissue.ReadOptions{Kind: c.kind})
			if !c.wantErr {
				if err != nil {
					t.Fatalf("Read: %v", err)
				}
				if read.Issue == nil || read.Issue.Index != idx {
					t.Fatalf("issue = %+v, want index %d", read.Issue, idx)
				}
				return
			}
			var refused *apppissue.KindError
			if !errors.As(err, &refused) {
				t.Fatalf("err = %v, want a *KindError", err)
			}
			if refused.Index != idx || refused.Want != c.want {
				t.Fatalf("KindError = %+v, want index %d want %q", refused, idx, c.want)
			}
		})
	}
}

func TestParseKind(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"", "issues", "pulls"} {
		if _, err := apppissue.ParseKind(s); err != nil {
			t.Errorf("ParseKind(%q): %v", s, err)
		}
	}
	for _, s := range []string{"issue", "Pulls", "pr", " issues"} {
		if _, err := apppissue.ParseKind(s); err == nil {
			t.Errorf("ParseKind(%q) accepted an unknown type", s)
		}
	}
}
