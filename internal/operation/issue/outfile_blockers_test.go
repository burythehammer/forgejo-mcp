// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/list"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// testLib is a second repository of testOwner, for cross-repository blockers.
const testLib = "lib"

// newHandlersFor serves the handlers from f, as newFixture does for a fresh fake.
func newHandlersFor(f *fake.Forgejo) handlers { return newHandlers(fake.Factory{Forgejo: f}) }

// addBlocker makes the issue from in testOwner/testRepo depend on blocker, which
// may be in any repository. The issues from depends on are its blockers.
func addBlocker(t *testing.T, f *fake.Forgejo, from int64, blocker forge.IssueRef) {
	t.Helper()
	if err := f.AddIssueDependency(context.Background(), testOwner, testRepo, from, blocker); err != nil {
		t.Fatalf("seeding dependency of %d on %+v: %v", from, blocker, err)
	}
}

// blockedRows reads the file at path and returns each row's blocked_by keyed by
// issue number, as the JSON holds it: numbers decode as float64 and a
// cross-repository blocker as a string. A row with no blocked_by key or a null
// fails the test, since the contract is always an array.
func blockedRows(t *testing.T, path string) map[int64][]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var rows []struct {
		Number    int64 `json:"number"`
		BlockedBy []any `json:"blocked_by"`
	}
	if err := json.Unmarshal(b, &rows); err != nil {
		t.Fatalf("file is not a JSON array of rows: %v", err)
	}
	out := map[int64][]any{}
	for _, r := range rows {
		if r.BlockedBy == nil {
			t.Fatalf("row %d has no blocked_by array", r.Number)
		}
		out[r.Number] = r.BlockedBy
	}
	return out
}

// sortedAny orders a blocked_by value so tests compare sets, not the order the
// fake happens to return; the golden pins that order separately.
func sortedAny(vs []any) []any {
	out := append([]any(nil), vs...)
	sort.SliceStable(out, func(i, j int) bool {
		return fmt.Sprint(out[i]) < fmt.Sprint(out[j])
	})
	return out
}

// depSpy counts the dependency reads a call makes, and otherwise serves them
// from the fake.
type depSpy struct {
	*fake.Forgejo
	reads int
}

func (s *depSpy) ListIssueDependencies(ctx context.Context, owner, repo string, index int64, pg, limit int) ([]*forgejo_sdk.Issue, error) {
	s.reads++
	return s.Forgejo.ListIssueDependencies(ctx, owner, repo, index, pg, limit)
}

func TestListRepoIssuesWithBlockers_NoBlockersIsEmptyArray(t *testing.T) {
	dir := outFileTempDir(t)
	h, f := newFixture(t)
	idx := seedOpen(t, f, 2)

	call(t, h.ListRepoIssuesFn, map[string]any{"outFile": "none.json", "with_blockers": true})
	got := blockedRows(t, filepath.Join(dir, "none.json"))
	for _, i := range idx {
		if len(got[i]) != 0 {
			t.Fatalf("issue %d blocked_by = %v, want []", i, got[i])
		}
	}
}

func TestListRepoIssuesWithBlockers_OneBlocker(t *testing.T) {
	dir := outFileTempDir(t)
	h, f := newFixture(t)
	idx := seedOpen(t, f, 2)
	addBlocker(t, f, idx[1], forge.IssueRef{Owner: testOwner, Repo: testRepo, Index: idx[0]})

	call(t, h.ListRepoIssuesFn, map[string]any{"outFile": "one.json", "with_blockers": true})
	got := blockedRows(t, filepath.Join(dir, "one.json"))
	if want := []any{float64(idx[0])}; !reflect.DeepEqual(got[idx[1]], want) {
		t.Fatalf("issue %d blocked_by = %v, want %v", idx[1], got[idx[1]], want)
	}
	if len(got[idx[0]]) != 0 {
		t.Fatalf("issue %d blocked_by = %v, want []", idx[0], got[idx[0]])
	}
}

func TestListRepoIssuesWithBlockers_SeveralBlockers(t *testing.T) {
	dir := outFileTempDir(t)
	h, f := newFixture(t)
	idx := seedOpen(t, f, 3)
	addBlocker(t, f, idx[2], forge.IssueRef{Owner: testOwner, Repo: testRepo, Index: idx[0]})
	addBlocker(t, f, idx[2], forge.IssueRef{Owner: testOwner, Repo: testRepo, Index: idx[1]})

	call(t, h.ListRepoIssuesFn, map[string]any{"outFile": "several.json", "with_blockers": true})
	got := blockedRows(t, filepath.Join(dir, "several.json"))
	want := []any{float64(idx[0]), float64(idx[1])}
	if !reflect.DeepEqual(sortedAny(got[idx[2]]), sortedAny(want)) {
		t.Fatalf("issue %d blocked_by = %v, want the set %v", idx[2], got[idx[2]], want)
	}
}

func TestListRepoIssuesWithBlockers_CrossRepoBlockerIsOwnerRepoHash(t *testing.T) {
	dir := outFileTempDir(t)
	h, f := newFixture(t)
	f.AddRepo(testOwner, testLib)
	idx := seedOpen(t, f, 1)
	lib, err := f.CreateIssue(context.Background(), testOwner, testLib, forgejo_sdk.CreateIssueOption{Title: "lib issue"})
	if err != nil {
		t.Fatal(err)
	}
	addBlocker(t, f, idx[0], forge.IssueRef{Owner: testOwner, Repo: testLib, Index: lib.Index})

	call(t, h.ListRepoIssuesFn, map[string]any{"outFile": "cross.json", "with_blockers": true})
	got := blockedRows(t, filepath.Join(dir, "cross.json"))
	want := []any{testOwner + "/" + testLib + "#" + itoa(lib.Index)}
	if !reflect.DeepEqual(got[idx[0]], want) {
		t.Fatalf("issue %d blocked_by = %v, want %v", idx[0], got[idx[0]], want)
	}
}

func TestListRepoIssuesWithBlockers_MixedSameAndCrossRepo(t *testing.T) {
	dir := outFileTempDir(t)
	h, f := newFixture(t)
	f.AddRepo(testOwner, testLib)
	idx := seedOpen(t, f, 2)
	lib, err := f.CreateIssue(context.Background(), testOwner, testLib, forgejo_sdk.CreateIssueOption{Title: "lib issue"})
	if err != nil {
		t.Fatal(err)
	}
	addBlocker(t, f, idx[1], forge.IssueRef{Owner: testOwner, Repo: testRepo, Index: idx[0]})
	addBlocker(t, f, idx[1], forge.IssueRef{Owner: testOwner, Repo: testLib, Index: lib.Index})

	call(t, h.ListRepoIssuesFn, map[string]any{"outFile": "mixed.json", "with_blockers": true})
	got := blockedRows(t, filepath.Join(dir, "mixed.json"))
	want := []any{float64(idx[0]), testOwner + "/" + testLib + "#" + itoa(lib.Index)}
	if !reflect.DeepEqual(sortedAny(got[idx[1]]), sortedAny(want)) {
		t.Fatalf("issue %d blocked_by = %v, want the set %v", idx[1], got[idx[1]], want)
	}
}

func TestListRepoIssuesWithBlockers_OneDependencyReadPerListedIssue(t *testing.T) {
	outFileTempDir(t)
	_, f := newFixture(t)
	seedOpen(t, f, 3)
	spy := &depSpy{Forgejo: f}
	h := withPort(spy)

	call(t, h.ListRepoIssuesFn, map[string]any{"outFile": "reads.json", "with_blockers": true})
	if spy.reads != 3 {
		t.Fatalf("dependency reads = %d, want one per listed issue (3)", spy.reads)
	}
}

func TestListRepoIssuesWithBlockers_RefusedWithoutOutFile(t *testing.T) {
	_, f := newFixture(t)
	seedOpen(t, f, 1)
	msg := callErr(t, newHandlersFor(f).ListRepoIssuesFn, map[string]any{"with_blockers": true})
	if !strings.Contains(msg, "with_blockers") || !strings.Contains(msg, "outFile") {
		t.Fatalf("with_blockers without outFile: error %q does not name both", msg)
	}
}

func TestListRepoIssuesWithBlockers_OverTheListingCeilingWritesNothingAndReadsNoBlockers(t *testing.T) {
	dir := outFileTempDir(t)
	// pagedStub's Forgejo is nil: a dependency read would panic, so a passing
	// run proves no blocker was read before the ceiling refused the listing.
	h := withPort(&pagedStub{n: list.FileCeiling + 1})
	msg := callErr(t, h.ListRepoIssuesFn, map[string]any{"outFile": "over.json", "with_blockers": true})
	if !strings.Contains(msg, "1000") || !strings.Contains(msg, "ceiling") {
		t.Fatalf("error %q does not name the ceiling", msg)
	}
	noFileWritten(t, dir)
}

func TestListRepoIssuesWithBlockers_FailedDependencyReadFailsTheCallAndWritesNothing(t *testing.T) {
	dir := outFileTempDir(t)
	h, f := newFixture(t)
	seedOpen(t, f, 3)
	f.Refuse("ListIssueDependencies", 500, "dependency service down")
	msg := callErr(t, h.ListRepoIssuesFn, map[string]any{"outFile": "partial.json", "with_blockers": true})
	if !strings.Contains(msg, "dependency") {
		t.Fatalf("error %q does not name the failed dependency read", msg)
	}
	noFileWritten(t, dir)
}

func TestSearchIssuesWithBlockers_NamesEachIssuesOwnRepository(t *testing.T) {
	dir := outFileTempDir(t)
	h, f := newFixture(t)
	f.AddRepo(testOwner, testLib)
	idx := seedOpen(t, f, 1)
	lib, err := f.CreateIssue(context.Background(), testOwner, testLib, forgejo_sdk.CreateIssueOption{Title: "lib issue"})
	if err != nil {
		t.Fatal(err)
	}
	// The searched issue in testRepo depends on the one in testLib: the
	// blocker is in another repository than the issue it blocks.
	addBlocker(t, f, idx[0], forge.IssueRef{Owner: testOwner, Repo: testLib, Index: lib.Index})

	searchOutCall(t, h, map[string]any{"outFile": "search.json", "with_blockers": true})
	got := blockedRows(t, filepath.Join(dir, "search.json"))
	want := []any{testOwner + "/" + testLib + "#" + itoa(lib.Index)}
	if !reflect.DeepEqual(got[idx[0]], want) {
		t.Fatalf("search row %d blocked_by = %v, want %v", idx[0], got[idx[0]], want)
	}
}

func TestSearchIssuesWithBlockers_RefusedWithoutOutFile(t *testing.T) {
	_, f := newFixture(t)
	seedOpen(t, f, 1)
	msg := searchOutErr(t, newHandlersFor(f), map[string]any{"with_blockers": true})
	if !strings.Contains(msg, "with_blockers") || !strings.Contains(msg, "outFile") {
		t.Fatalf("search with_blockers without outFile: error %q does not name both", msg)
	}
}

func TestSearchIssuesWithBlockers_FailedDependencyReadWritesNothing(t *testing.T) {
	dir := outFileTempDir(t)
	h, f := newFixture(t)
	seedOpen(t, f, 2)
	f.Refuse("ListIssueDependencies", 500, "dependency service down")
	msg := searchOutErr(t, h, map[string]any{"outFile": "search-fail.json", "with_blockers": true})
	if !strings.Contains(msg, "dependency") {
		t.Fatalf("error %q does not name the failed dependency read", msg)
	}
	noFileWritten(t, dir)
}
