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

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/list"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/mcptest"

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

// blockedByTitle is blockedRows keyed by title, for a search whose rows can share
// an issue number across repositories.
func blockedByTitle(t *testing.T, path string) map[string][]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var rows []struct {
		Title     string `json:"title"`
		BlockedBy []any  `json:"blocked_by"`
	}
	if err := json.Unmarshal(b, &rows); err != nil {
		t.Fatalf("file is not a JSON array of rows: %v", err)
	}
	out := map[string][]any{}
	for _, r := range rows {
		if r.BlockedBy == nil {
			t.Fatalf("row %q has no blocked_by array", r.Title)
		}
		out[r.Title] = r.BlockedBy
	}
	return out
}

func TestSearchIssuesWithBlockers_NamesEachIssuesOwnRepository(t *testing.T) {
	dir := outFileTempDir(t)
	h, f := newFixture(t)
	f.AddRepo(testOwner, testLib)
	ctx := context.Background()
	blocked, err := f.CreateIssue(ctx, testOwner, testRepo, forgejo_sdk.CreateIssueOption{Title: "search-blocked"})
	if err != nil {
		t.Fatal(err)
	}
	lib, err := f.CreateIssue(ctx, testOwner, testLib, forgejo_sdk.CreateIssueOption{Title: "lib issue"})
	if err != nil {
		t.Fatal(err)
	}
	// The issue in testRepo depends on the one in testLib: its blocker is in
	// another repository. The lib issue has no blockers of its own.
	addBlocker(t, f, blocked.Index, forge.IssueRef{Owner: testOwner, Repo: testLib, Index: lib.Index})

	searchOutCall(t, h, map[string]any{"outFile": "search.json", "with_blockers": true})
	got := blockedByTitle(t, filepath.Join(dir, "search.json"))
	if want := []any{testOwner + "/" + testLib + "#" + itoa(lib.Index)}; !reflect.DeepEqual(got["search-blocked"], want) {
		t.Fatalf("search-blocked blocked_by = %v, want %v", got["search-blocked"], want)
	}
	if len(got["lib issue"]) != 0 {
		t.Fatalf("lib issue blocked_by = %v, want []", got["lib issue"])
	}
}

// fixedListingStub serves three issues with distinct URLs and no pull request
// metadata, and fixed blockers, so the file goldens hold stable bytes for a
// realistic listing. Each list and search returns the three in one page.
type fixedListingStub struct {
	forge.Forgejo
	deps map[int64][]*forgejo_sdk.Issue
}

func (s *fixedListingStub) MaxResponseItems(context.Context) (int, bool) { return 0, false }

func (s *fixedListingStub) rows() []*forgejo_sdk.Issue {
	titles := []string{"first issue", "second issue", "third issue"}
	out := make([]*forgejo_sdk.Issue, 0, len(titles))
	for i, title := range titles {
		iss := goldenIssue(title)
		iss.ID = int64(i + 1)
		iss.Index = int64(i + 1)
		iss.URL = fmt.Sprintf("https://f.example/api/v1/repos/octo/hello/issues/%d", i+1)
		iss.HTMLURL = fmt.Sprintf("https://f.example/octo/hello/issues/%d", i+1)
		iss.PullRequest = nil
		out = append(out, iss)
	}
	return out
}

func (s *fixedListingStub) ListRepoIssues(_ context.Context, _, _ string, _ forge.ListIssuesOptions) ([]*forgejo_sdk.Issue, forge.PageInfo, error) {
	return s.rows(), forge.PageInfo{}, nil
}

func (s *fixedListingStub) SearchIssues(_ context.Context, _ forgejo_sdk.ListIssueOption) ([]*forgejo_sdk.Issue, forge.PageInfo, error) {
	return s.rows(), forge.PageInfo{}, nil
}

// ListIssueDependencies serves the fixed blockers of index, on its first page only.
func (s *fixedListingStub) ListIssueDependencies(_ context.Context, _, _ string, index int64, pg, _ int) ([]*forgejo_sdk.Issue, error) {
	if pg > 1 {
		return nil, nil
	}
	return s.deps[index], nil
}

// blockersFixture is the fixed listing: issue 2 is blocked by issue 1 of the
// listed repository, and issue 3 by issue 1 and by issue 9 of testLib.
func blockersFixture() *fixedListingStub {
	same := goldenIssue("blocker in this repository")
	same.Index = 1
	lib := goldenIssue("blocker in another repository")
	lib.Index = 9
	lib.Repository = &forgejo_sdk.RepositoryMeta{ID: 10, Name: testLib, Owner: testOwner, FullName: testOwner + "/" + testLib}
	return &fixedListingStub{deps: map[int64][]*forgejo_sdk.Issue{2: {same}, 3: {same, lib}}}
}

func TestListRepoIssuesWithBlockers_Golden(t *testing.T) {
	dir := outFileTempDir(t)
	h := withPort(blockersFixture())
	res, err := h.ListRepoIssuesFn(context.Background(), repoReq(map[string]any{"outFile": "blockers.json", "with_blockers": true}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("golden call failed: err=%v res=%+v", err, res)
	}
	file, err := os.ReadFile(filepath.Join(dir, "blockers.json"))
	if err != nil {
		t.Fatal(err)
	}
	mcptest.Golden(t, "list_repo_issues_blockers_file", file)
}

func TestSearchIssuesWithBlockers_Golden(t *testing.T) {
	dir := outFileTempDir(t)
	h := withPort(blockersFixture())
	searchOutCall(t, h, map[string]any{"outFile": "blockers-search.json", "with_blockers": true})
	file, err := os.ReadFile(filepath.Join(dir, "blockers-search.json"))
	if err != nil {
		t.Fatal(err)
	}
	mcptest.Golden(t, "search_issues_blockers_file", file)
}

func TestListRepoIssuesWithBlockers_VerboseGolden(t *testing.T) {
	dir := outFileTempDir(t)
	h := withPort(blockersFixture())
	call(t, h.ListRepoIssuesFn, map[string]any{"outFile": "blockers-verbose.json", "with_blockers": true, "verbose": true})
	file, err := os.ReadFile(filepath.Join(dir, "blockers-verbose.json"))
	if err != nil {
		t.Fatal(err)
	}
	mcptest.Golden(t, "list_repo_issues_blockers_verbose_file", file)
}

// manyBlockersStub lists one issue whose dependency list holds n issues, served
// page by page at the requested limit. It drives the per-issue blocker ceiling.
type manyBlockersStub struct {
	pagedStub
	n int
}

func (s *manyBlockersStub) ListIssueDependencies(_ context.Context, _, _ string, _ int64, pg, limit int) ([]*forgejo_sdk.Issue, error) {
	var out []*forgejo_sdk.Issue
	for i := (pg - 1) * limit; i < pg*limit && i < s.n; i++ {
		iss := goldenIssue("blocker")
		iss.Index = int64(i + 2)
		out = append(out, iss)
	}
	return out, nil
}

// An issue with exactly the file ceiling of blockers writes whole.
func TestListRepoIssuesWithBlockers_BlockersAtTheCeilingWriteWhole(t *testing.T) {
	dir := outFileTempDir(t)
	h := withPort(&manyBlockersStub{pagedStub: pagedStub{n: 1}, n: list.FileCeiling})
	call(t, h.ListRepoIssuesFn, map[string]any{"outFile": "at-ceiling.json", "with_blockers": true})
	got := blockedRows(t, filepath.Join(dir, "at-ceiling.json"))
	if len(got[1]) != list.FileCeiling {
		t.Fatalf("issue 1 blocked_by has %d entries, want %d", len(got[1]), list.FileCeiling)
	}
}

// An issue with one blocker more than the file ceiling fails the whole call and
// writes nothing, naming the blockers and the ceiling. The listing itself is one
// issue, so only the per-issue read can refuse it.
func TestListRepoIssuesWithBlockers_BlockersOverTheCeilingFailAndWriteNothing(t *testing.T) {
	dir := outFileTempDir(t)
	h := withPort(&manyBlockersStub{pagedStub: pagedStub{n: 1}, n: list.FileCeiling + 1})
	msg := callErr(t, h.ListRepoIssuesFn, map[string]any{"outFile": "over-blockers.json", "with_blockers": true})
	if !strings.Contains(msg, "blockers of octo/hello#1") || !strings.Contains(msg, "1000") || !strings.Contains(msg, "ceiling") {
		t.Fatalf("error %q does not name the blockers, the issue and the ceiling", msg)
	}
	noFileWritten(t, dir)
}

// verbose rows keep the full issue and add blocked_by; the blocker is named the
// same way as in the compact rows.
func TestListRepoIssuesWithBlockers_VerboseRowsCarryBlockedBy(t *testing.T) {
	dir := outFileTempDir(t)
	h, f := newFixture(t)
	idx := seedOpen(t, f, 2)
	addBlocker(t, f, idx[1], forge.IssueRef{Owner: testOwner, Repo: testRepo, Index: idx[0]})

	call(t, h.ListRepoIssuesFn, map[string]any{"outFile": "verbose.json", "with_blockers": true, "verbose": true})
	b, err := os.ReadFile(filepath.Join(dir, "verbose.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Number    int64  `json:"number"`
		Title     string `json:"title"`
		BlockedBy []any  `json:"blocked_by"`
	}
	if err := json.Unmarshal(b, &rows); err != nil {
		t.Fatalf("verbose file is not a JSON array of rows: %v", err)
	}
	got := map[int64]struct {
		title string
		by    []any
	}{}
	for _, r := range rows {
		if r.BlockedBy == nil {
			t.Fatalf("verbose row %d has no blocked_by array", r.Number)
		}
		got[r.Number] = struct {
			title string
			by    []any
		}{r.Title, r.BlockedBy}
	}
	if got[idx[1]].title != "issue" || !reflect.DeepEqual(got[idx[1]].by, []any{float64(idx[0])}) {
		t.Fatalf("verbose row %d = %+v, want title %q and blocked_by [%d]", idx[1], got[idx[1]], "issue", idx[0])
	}
	if len(got[idx[0]].by) != 0 {
		t.Fatalf("verbose row %d blocked_by = %v, want []", idx[0], got[idx[0]].by)
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
