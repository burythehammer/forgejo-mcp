// SPDX-License-Identifier: GPL-3.0-or-later

package branchprotection_test

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	appbp "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/branchprotection"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge/branchprotectionadapter"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge/fake"
)

// The use cases run against the in-memory fake through the branch protection
// adapter: no MCP, no HTTP. The fake is built per test, so tests run in
// parallel.

const (
	owner = "octo"
	repo  = "hello"
)

func newService(t *testing.T) *appbp.Service {
	t.Helper()
	f := fake.New()
	f.AddRepo(owner, repo)
	return appbp.New(branchprotectionadapter.New(f))
}

func ref(rule string) appbp.RuleRef { return appbp.RuleRef{Owner: owner, Repo: repo, Rule: rule} }

func seed(t *testing.T, s *appbp.Service, branch string) {
	t.Helper()
	if _, err := s.Create(context.Background(), appbp.CreateRequest{Owner: owner, Repo: repo, BranchName: branch}); err != nil {
		t.Fatalf("seeding %s: %v", branch, err)
	}
}

func str(s string) *string { return &s }

func TestListPagesThroughRules(t *testing.T) {
	t.Parallel()
	s := newService(t)
	for _, b := range []string{"main", "dev", "release"} {
		seed(t, s, b)
	}
	first, err := s.List(context.Background(), appbp.ListRequest{Owner: owner, Repo: repo, Page: 1, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if first.Count != 2 || first.Page != 1 || first.Limit != 2 || len(first.Items) != 2 {
		t.Fatalf("page 1: got %+v", first)
	}
	second, err := s.List(context.Background(), appbp.ListRequest{Owner: owner, Repo: repo, Page: 2, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if second.Count != 1 {
		t.Fatalf("page 2: got count %d, want 1", second.Count)
	}
}

func TestListDefaultsToPageOneOf100(t *testing.T) {
	t.Parallel()
	s := newService(t)
	got, err := s.List(context.Background(), appbp.ListRequest{Owner: owner, Repo: repo})
	if err != nil {
		t.Fatal(err)
	}
	if got.Page != 1 || got.Limit != 100 {
		t.Fatalf("got page %d limit %d, want 1 and 100", got.Page, got.Limit)
	}
}

func TestListZeroPageTakesThePageOne(t *testing.T) {
	t.Parallel()
	s := newService(t)
	// The adapter has already validated the numbers; a zero page is page 1.
	got, err := s.List(context.Background(), appbp.ListRequest{Owner: owner, Repo: repo, Page: 0, Limit: 3})
	if err != nil {
		t.Fatal(err)
	}
	if got.Page != 1 || got.Limit != 3 {
		t.Fatalf("got page %d limit %d, want 1 and 3", got.Page, got.Limit)
	}
}

func TestMissingOwnerIsRejected(t *testing.T) {
	t.Parallel()
	s := newService(t)
	_, err := s.List(context.Background(), appbp.ListRequest{Repo: repo})
	if err == nil || err.Error() != "owner and repo are required" {
		t.Fatalf("got %v", err)
	}
}

func TestGetUnknownIsWrapped(t *testing.T) {
	t.Parallel()
	s := newService(t)
	_, err := s.Get(context.Background(), ref("nope"))
	if err == nil || !strings.HasPrefix(err.Error(), "get branch protection err: ") {
		t.Fatalf("got %v", err)
	}
}

func TestCreateRoundTripsStatusChecks(t *testing.T) {
	t.Parallel()
	s := newService(t)
	bp, err := s.Create(context.Background(), appbp.CreateRequest{
		Owner: owner, Repo: repo, BranchName: "main", EnableStatusCheck: true,
		StatusCheckContexts: str("ci/build, lint,,"), RequiredApprovals: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"ci/build", "lint"}
	if !reflect.DeepEqual(bp.StatusCheckContexts, want) || bp.RequiredApprovals != 2 {
		t.Fatalf("created: contexts %v approvals %d", bp.StatusCheckContexts, bp.RequiredApprovals)
	}
}

func TestCreateNeedsBranchName(t *testing.T) {
	t.Parallel()
	s := newService(t)
	_, err := s.Create(context.Background(), appbp.CreateRequest{Owner: owner, Repo: repo})
	if err == nil || err.Error() != "branch_name is required" {
		t.Fatalf("got %v", err)
	}
}

func TestEditChangesOnlyTheFieldsPassed(t *testing.T) {
	t.Parallel()
	s := newService(t)
	if _, err := s.Create(context.Background(), appbp.CreateRequest{
		Owner: owner, Repo: repo, BranchName: "main", EnablePush: true, RequiredApprovals: 1,
	}); err != nil {
		t.Fatal(err)
	}
	bp, err := s.Edit(context.Background(), appbp.EditRequest{
		RuleRef: ref("main"), EnableStatusCheck: func() *bool { v := true; return &v }(),
		StatusCheckContexts: str("ci/build"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bp.EnablePush || bp.RequiredApprovals != 1 || !bp.EnableStatusCheck || !reflect.DeepEqual(bp.StatusCheckContexts, []string{"ci/build"}) {
		t.Fatalf("edited: %+v", bp)
	}
}

func TestEditEmptyListClearsIt(t *testing.T) {
	t.Parallel()
	s := newService(t)
	if _, err := s.Create(context.Background(), appbp.CreateRequest{
		Owner: owner, Repo: repo, BranchName: "main", StatusCheckContexts: str("ci/build"),
	}); err != nil {
		t.Fatal(err)
	}
	bp, err := s.Edit(context.Background(), appbp.EditRequest{RuleRef: ref("main"), StatusCheckContexts: str("")})
	if err != nil {
		t.Fatal(err)
	}
	if len(bp.StatusCheckContexts) != 0 {
		t.Fatalf("contexts not cleared: %v", bp.StatusCheckContexts)
	}
}

func TestDeleteRemovesTheRule(t *testing.T) {
	t.Parallel()
	s := newService(t)
	seed(t, s, "main")
	if err := s.Delete(context.Background(), ref("main")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(context.Background(), ref("main")); err == nil {
		t.Fatal("rule still readable after delete")
	}
}

func TestListHasNextMarksTheLastPage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		n       int
		page    int
		count   int
		hasNext bool
	}{
		{"full page with more rules", 5, 1, 2, true},
		{"exact multiple, full last page", 4, 2, 2, false},
		{"short last page", 5, 3, 1, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := newService(t)
			for i := range c.n {
				seed(t, s, fmt.Sprintf("b%d", i+1))
			}
			got, err := s.List(context.Background(), appbp.ListRequest{Owner: owner, Repo: repo, Page: c.page, Limit: 2})
			if err != nil || got.Count != c.count || got.HasNext != c.hasNext {
				t.Fatalf("got %+v, %v", got, err)
			}
		})
	}
}
