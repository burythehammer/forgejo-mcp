// SPDX-License-Identifier: GPL-3.0-or-later

package repo_test

import (
	"context"
	"fmt"
	"testing"

	apprepo "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/repo"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/repoadminadapter"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/repogitadapter"
)

const (
	readOwner = "octo"
	readRepo  = "hello"
	readSHA   = "a3f1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9"
)

// topicsFail is an admin port whose topic listing fails. A read that does not
// ask for topics must never list them.
type topicsFail struct{ apprepo.AdminPort }

func (topicsFail) ListRepoTopics(context.Context, string, string, apprepo.AdminPaging) ([]string, error) {
	return nil, fmt.Errorf("topics boom")
}

func readFixture(t *testing.T) (*apprepo.AdminService, *fake.Forgejo) {
	t.Helper()
	f := fake.New()
	f.AddRepo(readOwner, readRepo)
	return apprepo.NewAdmin(repoadminadapter.New(f)), f
}

// seedCommit makes readSHA a known commit with the given message.
func seedCommit(t *testing.T, f *fake.Forgejo, sha, message string) {
	t.Helper()
	if err := f.AddCommit(readOwner, readRepo, sha, message); err != nil {
		t.Fatalf("seeding commit: %v", err)
	}
}

func TestReadRepo_TopicsNotListedUnlessAsked(t *testing.T) {
	t.Parallel()
	_, f := readFixture(t)
	s := apprepo.NewAdmin(topicsFail{repoadminadapter.New(f)})

	read, err := s.ReadRepo(context.Background(), readOwner, readRepo, apprepo.RepoReadOptions{})
	if err != nil {
		t.Fatalf("ReadRepo: %v", err)
	}
	if read.Repo == nil || read.Repo.Name != readRepo {
		t.Fatalf("repo = %+v", read.Repo)
	}
	if read.Topics != nil {
		t.Fatalf("topics = %v, want none", read.Topics)
	}
}

func TestReadRepo_TopicsWhenAsked(t *testing.T) {
	t.Parallel()
	s, f := readFixture(t)
	if err := f.SetRepoTopics(context.Background(), readOwner, readRepo, []string{"go", "mcp"}); err != nil {
		t.Fatal(err)
	}

	read, err := s.ReadRepo(context.Background(), readOwner, readRepo, apprepo.RepoReadOptions{Topics: true})
	if err != nil {
		t.Fatalf("ReadRepo: %v", err)
	}
	if len(read.Topics) != 2 || read.Topics[0] != "go" || read.Topics[1] != "mcp" {
		t.Fatalf("topics = %v, want [go mcp]", read.Topics)
	}
}

func TestReadRepo_MissingRepoIsAnError(t *testing.T) {
	t.Parallel()
	s, _ := readFixture(t)
	if _, err := s.ReadRepo(context.Background(), readOwner, "nope", apprepo.RepoReadOptions{}); err == nil {
		t.Fatal("want an error for a missing repository")
	}
}

func TestReadCommitStatuses_AggregateState(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		states []string
		want   string
	}{
		{"none is unknown", nil, "unknown"},
		{"all success", []string{"success", "success"}, "success"},
		{"pending beats success", []string{"success", "pending"}, "pending"},
		{"failure beats pending", []string{"pending", "failure"}, "failure"},
		{"error counts as failure", []string{"success", "error"}, "failure"},
		{"warning is unknown", []string{"success", "warning"}, "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s, f := readFixture(t)
			seedCommit(t, f, readSHA, "msg")
			for i, st := range tc.states {
				if err := f.AddCommitStatusState(readOwner, readRepo, readSHA, fmt.Sprint("ci/", i), st, "d"); err != nil {
					t.Fatal(err)
				}
			}
			sum, err := s.CommitStatusSummary(context.Background(), readOwner, readRepo, readSHA)
			if err != nil {
				t.Fatalf("CommitStatusSummary: %v", err)
			}
			if sum.State != tc.want {
				t.Fatalf("state = %q, want %q", sum.State, tc.want)
			}
		})
	}
}

func TestGetCommitStatuses_PageHasNextAndTotal(t *testing.T) {
	t.Parallel()
	s, f := readFixture(t)
	seedCommit(t, f, readSHA, "msg")
	for i := 0; i < 40; i++ {
		if err := f.AddCommitStatusState(readOwner, readRepo, readSHA, fmt.Sprint("ci/", i), "success", "d"); err != nil {
			t.Fatal(err)
		}
	}

	read, err := s.GetCommitStatuses(context.Background(), readOwner, readRepo, readSHA, apprepo.AdminPaging{Page: 1, Limit: 31})
	if err != nil {
		t.Fatalf("GetCommitStatuses: %v", err)
	}
	if len(read.Items) != 31 {
		t.Fatalf("statuses = %d, want the 31 asked for", len(read.Items))
	}
	if read.Total == nil || *read.Total != 40 {
		t.Fatalf("total = %v, want 40 (the grand total, not the page)", read.Total)
	}
	if !read.HasNext || read.Page != 1 || read.Limit != 31 || read.Count != 31 {
		t.Fatalf("envelope = page %d limit %d count %d has_next %v", read.Page, read.Limit, read.Count, read.HasNext)
	}
}

func TestCommitStatusSummary_FailureBeyondFirstPage(t *testing.T) {
	t.Parallel()
	s, f := readFixture(t)
	seedCommit(t, f, readSHA, "msg")
	for i := 0; i < 40; i++ {
		st := "success"
		if i == 34 { // the 35th status, past the first page of 31
			st = "failure"
		}
		if err := f.AddCommitStatusState(readOwner, readRepo, readSHA, fmt.Sprint("ci/", i), st, "d"); err != nil {
			t.Fatal(err)
		}
	}

	sum, err := s.CommitStatusSummary(context.Background(), readOwner, readRepo, readSHA)
	if err != nil {
		t.Fatalf("CommitStatusSummary: %v", err)
	}
	if sum.State != "failure" {
		t.Fatalf("state = %q, want failure (the 35th status)", sum.State)
	}
	if sum.TotalCount != 40 || len(sum.Statuses) != 40 {
		t.Fatalf("total = %d, statuses = %d, want 40 and 40", sum.TotalCount, len(sum.Statuses))
	}
	if !sum.Complete {
		t.Fatal("complete = false, want true when every status was read")
	}
}

func TestCommitStatusSummary_TooLargeToReadIsUnknown(t *testing.T) {
	t.Parallel()
	s, f := readFixture(t)
	seedCommit(t, f, readSHA, "msg")
	// One more than the read ceiling (20 pages of 50): every status is success,
	// but the read cannot see them all, so no state may be claimed.
	const total = 1001
	for i := 0; i < total; i++ {
		if err := f.AddCommitStatusState(readOwner, readRepo, readSHA, fmt.Sprint("ci/", i), "success", "d"); err != nil {
			t.Fatal(err)
		}
	}

	sum, err := s.CommitStatusSummary(context.Background(), readOwner, readRepo, readSHA)
	if err != nil {
		t.Fatalf("CommitStatusSummary: %v", err)
	}
	if sum.State != "unknown" {
		t.Fatalf("state = %q, want unknown when the statuses cannot all be read", sum.State)
	}
	if sum.Complete {
		t.Fatal("complete = true, want false")
	}
	if sum.TotalCount != total {
		t.Fatalf("total = %d, want the real total %d", sum.TotalCount, total)
	}
}

func TestReadCommit_ReturnsTheCommit(t *testing.T) {
	t.Parallel()
	_, f := readFixture(t)
	seedCommit(t, f, readSHA, "fix: resolve race")
	g := apprepo.NewGitService(repogitadapter.New(f))

	c, err := g.ReadCommit(context.Background(), readOwner, readRepo, readSHA)
	if err != nil {
		t.Fatalf("ReadCommit: %v", err)
	}
	if c.SHA != readSHA {
		t.Fatalf("sha = %q, want %q", c.SHA, readSHA)
	}
	if c.RepoCommit == nil || c.RepoCommit.Message != "fix: resolve race" {
		t.Fatalf("commit = %+v, want the seeded message", c.RepoCommit)
	}
}

func TestReadCommit_UnknownIsAnError(t *testing.T) {
	t.Parallel()
	_, f := readFixture(t)
	g := apprepo.NewGitService(repogitadapter.New(f))
	if _, err := g.ReadCommit(context.Background(), readOwner, readRepo, readSHA); err == nil {
		t.Fatal("want an error for an unknown commit")
	}
}
