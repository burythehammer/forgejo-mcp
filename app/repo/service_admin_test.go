// SPDX-License-Identifier: GPL-3.0-or-later

package repo_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	apprepo "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/repo"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/fake"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/repoadminadapter"
)

// The admin use cases run against the in-memory fake through the bridge: no
// MCP, no HTTP. The fake is built per test, so tests run in parallel. Commit
// statuses need SDK types to seed and are covered by the bridge's tests.

const (
	owner = "octo"
	repo  = "hello"
	sha   = "0123456789abcdef0123456789abcdef01234567"
)

func newAdmin(t *testing.T) (*apprepo.AdminService, *fake.Forgejo) {
	t.Helper()
	f := fake.New()
	f.AddRepo(owner, repo)
	return apprepo.NewAdmin(repoadminadapter.New(f)), f
}

func str(s string) *string { return &s }

func wantErr(t *testing.T, err error, prefix string) {
	t.Helper()
	if err == nil || !strings.HasPrefix(err.Error(), prefix) {
		t.Fatalf("error %v, want prefix %q", err, prefix)
	}
}

func TestCreateRepoNeedsAName(t *testing.T) {
	t.Parallel()
	s, _ := newAdmin(t)
	_, err := s.CreateRepo(context.Background(), apprepo.CreateRepoInput{Owner: ""})
	if err == nil || err.Error() != "repository name is required" {
		t.Fatalf("got %v", err)
	}
}

func TestCreateRepoReturnsTheRepo(t *testing.T) {
	t.Parallel()
	s, _ := newAdmin(t)
	r, err := s.CreateRepo(context.Background(), apprepo.CreateRepoInput{Name: str("fresh"), Description: "d"})
	if err != nil || r.Name != "fresh" || r.Description != "d" {
		t.Fatalf("got %+v, %v", r, err)
	}
}

func TestGetRepoReturnsItsTopics(t *testing.T) {
	t.Parallel()
	s, _ := newAdmin(t)
	ctx := context.Background()
	if _, err := s.AddRepoTopic(ctx, owner, repo, "go"); err != nil {
		t.Fatal(err)
	}
	v, err := s.GetRepo(ctx, owner, repo)
	if err != nil || v.Name != repo || len(v.Topics) != 1 || v.Topics[0] != "go" {
		t.Fatalf("got %+v, %v", v, err)
	}
}

func TestGetRepoWithNoTopicsHasAnEmptyList(t *testing.T) {
	t.Parallel()
	s, _ := newAdmin(t)
	v, err := s.GetRepo(context.Background(), owner, repo)
	if err != nil || v.Topics == nil || len(v.Topics) != 0 {
		t.Fatalf("got %+v, %v", v, err)
	}
}

func TestGetUnknownRepoIsWrapped(t *testing.T) {
	t.Parallel()
	s, _ := newAdmin(t)
	_, err := s.GetRepo(context.Background(), owner, "nope")
	wantErr(t, err, "get repo: ")
}

func TestGetRepoNeedsOwnerAndRepo(t *testing.T) {
	t.Parallel()
	s, _ := newAdmin(t)
	_, err := s.GetRepo(context.Background(), "", repo)
	if err == nil || err.Error() != "owner and repo are required" {
		t.Fatalf("got %v", err)
	}
}

func TestEditRepoNeedsASetting(t *testing.T) {
	t.Parallel()
	s, _ := newAdmin(t)
	ctx := context.Background()
	for name, edit := range map[string]apprepo.RepoEdit{
		"nothing":    {},
		"empty name": {Name: str("")},
		"empty ref":  {DefaultBranch: str("")},
	} {
		_, err := s.EditRepo(ctx, apprepo.EditRepoInput{Owner: owner, Repo: repo, Edit: edit})
		if err == nil || err.Error() != "edit_repo: at least one setting field must be provided" {
			t.Fatalf("%s: got %v", name, err)
		}
	}
}

func TestEditRepoChangesOnlyWhatIsGiven(t *testing.T) {
	t.Parallel()
	s, _ := newAdmin(t)
	ctx := context.Background()
	r, err := s.EditRepo(ctx, apprepo.EditRepoInput{Owner: owner, Repo: repo, Edit: apprepo.RepoEdit{Description: str("new"), Archived: func() *bool { b := true; return &b }()}})
	if err != nil || r.Description != "new" || !r.Archived || r.Name != repo {
		t.Fatalf("got %+v, %v", r, err)
	}
}

func TestEditRepoRenames(t *testing.T) {
	t.Parallel()
	s, _ := newAdmin(t)
	ctx := context.Background()
	if _, err := s.EditRepo(ctx, apprepo.EditRepoInput{Owner: owner, Repo: repo, Edit: apprepo.RepoEdit{Name: str("renamed")}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetRepo(ctx, owner, "renamed"); err != nil {
		t.Fatalf("renamed repo not found: %v", err)
	}
}

func TestSetTopicsNormalisesAndDeduplicates(t *testing.T) {
	t.Parallel()
	s, _ := newAdmin(t)
	ctx := context.Background()
	got, err := s.SetRepoTopics(ctx, owner, repo, str("go, MCP, go,, ci"))
	if err != nil || strings.Join(got, " ") != "go mcp ci" {
		t.Fatalf("got %v, %v", got, err)
	}
	page, err := s.ListRepoTopics(ctx, owner, repo, apprepo.AdminPaging{Page: 1, Limit: 100})
	if err != nil || page.Count != 3 || page.Page != 1 || page.Limit != 100 {
		t.Fatalf("got %+v, %v", page, err)
	}
}

func TestSetTopicsEmptyClears(t *testing.T) {
	t.Parallel()
	s, _ := newAdmin(t)
	ctx := context.Background()
	if _, err := s.SetRepoTopics(ctx, owner, repo, str("go")); err != nil {
		t.Fatal(err)
	}
	got, err := s.SetRepoTopics(ctx, owner, repo, str(""))
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestSetTopicsRejectsBadInputAndChangesNothing(t *testing.T) {
	t.Parallel()
	s, _ := newAdmin(t)
	ctx := context.Background()
	if _, err := s.SetRepoTopics(ctx, owner, repo, nil); err == nil || err.Error() != "topics is required" {
		t.Fatalf("missing key: %v", err)
	}
	_, err := s.SetRepoTopics(ctx, owner, repo, str("ok,Bad Name"))
	wantErr(t, err, `topic "bad name" is invalid`)
	_, err = s.SetRepoTopics(ctx, owner, repo, str(strings.Repeat("a", 36)))
	wantErr(t, err, "topic ")
	tooMany := make([]string, 26)
	for i := range tooMany {
		tooMany[i] = "t" + string(rune('a'+i)) // distinct, so none is folded away
	}
	_, err = s.SetRepoTopics(ctx, owner, repo, str(strings.Join(tooMany, ",")))
	if err == nil || !strings.HasPrefix(err.Error(), "at most 25 topics per repository, got ") {
		t.Fatalf("too many: %v", err)
	}
	page, err := s.ListRepoTopics(ctx, owner, repo, apprepo.AdminPaging{Page: 1, Limit: 100})
	if err != nil || page.Count != 0 {
		t.Fatalf("topics changed: %+v, %v", page, err)
	}
}

func TestAddAndDeleteTopicReturnTheNormalisedName(t *testing.T) {
	t.Parallel()
	s, _ := newAdmin(t)
	ctx := context.Background()
	name, err := s.AddRepoTopic(ctx, owner, repo, "  CI  ")
	if err != nil || name != "ci" {
		t.Fatalf("add: %q, %v", name, err)
	}
	name, err = s.DeleteRepoTopic(ctx, owner, repo, "CI")
	if err != nil || name != "ci" {
		t.Fatalf("delete: %q, %v", name, err)
	}
	_, err = s.DeleteRepoTopic(ctx, owner, repo, "ci")
	wantErr(t, err, "delete repo topic: ")
}

func TestAddTopicRejectsInvalidName(t *testing.T) {
	t.Parallel()
	s, _ := newAdmin(t)
	_, err := s.AddRepoTopic(context.Background(), owner, repo, "Bad Name")
	wantErr(t, err, `topic "bad name" is invalid`)
	_, err = s.AddRepoTopic(context.Background(), owner, repo, "  ")
	if err == nil || err.Error() != "topic name is empty" {
		t.Fatalf("empty: %v", err)
	}
}

func TestListRepoTopicsNeedsOwnerAndRepo(t *testing.T) {
	t.Parallel()
	s, _ := newAdmin(t)
	_, err := s.ListRepoTopics(context.Background(), owner, "", apprepo.AdminPaging{Page: 1, Limit: 100})
	if err == nil || err.Error() != "owner and repo are required" {
		t.Fatalf("got %v", err)
	}
}

func TestCommitStatusesPageAndLimitAreClamped(t *testing.T) {
	t.Parallel()
	s, f := newAdmin(t)
	ctx := context.Background()
	if err := f.AddCommit(owner, repo, sha, "msg"); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name                string
		page, limit         int
		wantPage, wantLimit int
	}{
		{"zero takes defaults", 0, 0, 1, 30},
		{"negative takes defaults", -4, -4, 1, 30},
		{"above the maximum is clamped", 2, 999, 2, 50},
		{"in range is kept", 3, 7, 3, 7},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := s.GetCommitStatuses(ctx, owner, repo, sha, c.page, c.limit)
			if err != nil || got.Page != c.wantPage || got.Limit != c.wantLimit || got.Count != 0 || got.Statuses == nil || got.SHA != sha {
				t.Fatalf("got %+v, %v", got, err)
			}
		})
	}
}

func TestCommitStatusesValidatesTheSHAAfterTheRepo(t *testing.T) {
	t.Parallel()
	s, _ := newAdmin(t)
	ctx := context.Background()
	_, err := s.GetCommitStatuses(ctx, owner, repo, "abc", 1, 30)
	if err == nil || err.Error() != "invalid params: sha must be exactly 40 hex characters, got 3" {
		t.Fatalf("short: %v", err)
	}
	_, err = s.GetCommitStatuses(ctx, owner, repo, strings.Repeat("g", 40), 1, 30)
	if err == nil || err.Error() != `invalid params: sha contains invalid character 'g'` {
		t.Fatalf("bad char: %v", err)
	}
	_, err = s.GetCommitStatuses(ctx, "", repo, "abc", 1, 30)
	if err == nil || err.Error() != "owner and repo are required" {
		t.Fatalf("order: %v", err)
	}
}

func TestCommitStatusesUnknownCommitIsWrapped(t *testing.T) {
	t.Parallel()
	s, _ := newAdmin(t)
	_, err := s.GetCommitStatuses(context.Background(), owner, repo, sha, 1, 30)
	wantErr(t, err, "get commit statuses: ")
}

func TestListRepoTopicsPagesReportTheNextMarker(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name          string
		n, page, want int
		hasNext       bool
	}{
		{"full page with more topics", 5, 1, 2, true},
		{"middle page", 5, 2, 2, true},
		{"short last page", 5, 3, 1, false},
		{"exact multiple, full last page", 4, 2, 2, false},
		{"past the end", 4, 3, 0, false},
	}
	ctx := context.Background()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s, _ := newAdmin(t)
			names := make([]string, c.n)
			for i := range names {
				names[i] = fmt.Sprintf("topic%d", i+1)
			}
			if _, err := s.SetRepoTopics(ctx, owner, repo, str(strings.Join(names, ","))); err != nil {
				t.Fatal(err)
			}
			page, err := s.ListRepoTopics(ctx, owner, repo, apprepo.AdminPaging{Page: c.page, Limit: 2})
			if err != nil || len(page.Topics) != c.want || page.Count != c.want || page.HasNext != c.hasNext || page.Page != c.page || page.Limit != 2 {
				t.Fatalf("topics: %+v, %v", page, err)
			}
		})
	}
}
