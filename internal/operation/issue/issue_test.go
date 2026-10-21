// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"encoding/json"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcptest"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// unmarshalResult decodes a tool's text, which internal/to wraps as {"Result": ...}.
func unmarshalResult(text string, into any) error {
	var wrapped struct{ Result json.RawMessage }
	if err := json.Unmarshal([]byte(text), &wrapped); err != nil {
		return err
	}
	return json.Unmarshal(wrapped.Result, into)
}

func decodeIssue(t *testing.T, text string) forgejo_sdk.Issue {
	t.Helper()
	var iss forgejo_sdk.Issue
	if err := unmarshalResult(text, &iss); err != nil {
		t.Fatalf("not an issue: %v\n%s", err, text)
	}
	return iss
}

func assigneeNames(iss forgejo_sdk.Issue) []string {
	out := []string{}
	for _, a := range iss.Assignees {
		out = append(out, a.UserName)
	}
	return out
}

func TestGetIssueByIndex(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := openIssue(t, f, "hello")
	iss := decodeIssue(t, call(t, h.GetIssueByIndexFn, map[string]any{"index": float64(idx)}))
	if iss.Title != "hello" || iss.Index != idx {
		t.Fatalf("got %+v", iss)
	}
	if msg := callErr(t, h.GetIssueByIndexFn, map[string]any{"index": float64(99)}); !strings.Contains(msg, "get issue err") {
		t.Fatalf("error = %q", msg)
	}
}

// resultKeys returns the sorted keys of the object a tool wrapped in {"Result": ...}.
func resultKeys(t *testing.T, text string) []string {
	t.Helper()
	var wrapped struct{ Result map[string]json.RawMessage }
	if err := json.Unmarshal([]byte(text), &wrapped); err != nil {
		t.Fatalf("not a result envelope: %v\n%s", err, text)
	}
	keys := make([]string, 0, len(wrapped.Result))
	for k := range wrapped.Result {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// TestGetIssueByIndex_CompactByDefault: a call without verbose returns the
// compact read (the write shape plus body, closed_at, ref and comments) and
// no user object; verbose=true returns the full entity.
func TestGetIssueByIndex_CompactByDefault(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := openIssue(t, f, "hello")
	args := map[string]any{"index": float64(idx)}

	want := []string{"assignees", "body", "closed_at", "comments", "labels", "number", "ref", "state", "title", "updated_at", "url"}
	if got := resultKeys(t, call(t, h.GetIssueByIndexFn, args)); !reflect.DeepEqual(got, want) {
		t.Fatalf("default keys = %v, want %v", got, want)
	}

	args["verbose"] = true
	if got := resultKeys(t, call(t, h.GetIssueByIndexFn, args)); !slices.Contains(got, "user") || !slices.Contains(got, "repository") {
		t.Fatalf("verbose keys = %v, want the full entity with user and repository", got)
	}
}

func TestGetIssueByIndex_ReturnsRefAndDueDate(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	due := time.Date(2030, 5, 6, 0, 0, 0, 0, time.UTC)
	iss, err := f.CreateIssue(context.Background(), testOwner, testRepo, forgejo_sdk.CreateIssueOption{Title: "t", Ref: "refs/heads/dev", Deadline: &due})
	if err != nil {
		t.Fatal(err)
	}
	// The due date is verbose-only; the compact read carries the ref.
	got := decodeIssue(t, call(t, h.GetIssueByIndexFn, map[string]any{"index": float64(iss.Index), "verbose": true}))
	if got.Ref != "refs/heads/dev" || got.Deadline == nil || !got.Deadline.Equal(due) {
		t.Fatalf("ref=%q deadline=%v", got.Ref, got.Deadline)
	}
}

func TestCreateIssue(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	ms := f.SeedMilestone(testOwner, testRepo, "v1")
	iss := decodeIssue(t, call(t, h.CreateIssueFn, map[string]any{
		"title": "new", "body": "b", "ref": "refs/heads/x", "assignees": "alice, bob", "milestone": "1",
		"verbose": true,
	}))
	if iss.Title != "new" || iss.Body != "b" || iss.Ref != "refs/heads/x" {
		t.Fatalf("got %+v", iss)
	}
	if got := assigneeNames(iss); len(got) != 2 || got[0] != "alice" || got[1] != "bob" {
		t.Fatalf("assignees %v", got)
	}
	if iss.Milestone == nil || iss.Milestone.ID != ms {
		t.Fatalf("milestone %+v", iss.Milestone)
	}
}

func TestCreateIssue_InvalidMilestone(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	if msg := callErr(t, h.CreateIssueFn, map[string]any{"title": "t", "milestone": "abc"}); !strings.Contains(msg, "invalid milestone ID") {
		t.Fatalf("error = %q", msg)
	}
}

func TestUpdateIssue_Assignees(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := openIssue(t, f, "t")
	idxF := float64(idx)

	iss := decodeIssue(t, call(t, h.UpdateIssueFn, map[string]any{"index": idxF, "assignee": "goern", "verbose": true}))
	if got := assigneeNames(iss); len(got) != 1 || got[0] != "goern" {
		t.Fatalf("singular: %v", got)
	}
	iss = decodeIssue(t, call(t, h.UpdateIssueFn, map[string]any{"index": idxF, "assignees": "alice, bob ,carol", "verbose": true}))
	if got := assigneeNames(iss); strings.Join(got, ",") != "alice,bob,carol" {
		t.Fatalf("csv: %v", got)
	}
	iss = decodeIssue(t, call(t, h.UpdateIssueFn, map[string]any{"index": idxF, "assignees": "", "assignee": "ignored", "verbose": true}))
	if got := assigneeNames(iss); len(got) != 0 {
		t.Fatalf("empty clears: %v", got)
	}
	call(t, h.UpdateIssueFn, map[string]any{"index": idxF, "assignees": "keep"})
	iss = decodeIssue(t, call(t, h.UpdateIssueFn, map[string]any{"index": idxF, "title": "renamed", "verbose": true}))
	if got := assigneeNames(iss); len(got) != 1 || got[0] != "keep" || iss.Title != "renamed" {
		t.Fatalf("untouched assignees: %v title %q", got, iss.Title)
	}
}

func TestUpdateIssue_Ref(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := float64(openIssue(t, f, "t"))
	iss := decodeIssue(t, call(t, h.UpdateIssueFn, map[string]any{"index": idx, "ref": "refs/heads/a", "verbose": true}))
	if iss.Ref != "refs/heads/a" {
		t.Fatalf("set: %q", iss.Ref)
	}
	iss = decodeIssue(t, call(t, h.UpdateIssueFn, map[string]any{"index": idx, "title": "x", "verbose": true}))
	if iss.Ref != "refs/heads/a" {
		t.Fatalf("omitted ref changed it: %q", iss.Ref)
	}
	iss = decodeIssue(t, call(t, h.UpdateIssueFn, map[string]any{"index": idx, "ref": "", "verbose": true}))
	if iss.Ref != "" {
		t.Fatalf("empty ref should clear, got %q", iss.Ref)
	}
}

func TestUpdateIssue_DueDate(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := float64(openIssue(t, f, "t"))
	iss := decodeIssue(t, call(t, h.UpdateIssueFn, map[string]any{"index": idx, "due_date": "2030-01-02T03:04:05Z", "verbose": true}))
	if iss.Deadline == nil || iss.Deadline.Format(time.RFC3339) != "2030-01-02T03:04:05Z" {
		t.Fatalf("set: %v", iss.Deadline)
	}
	iss = decodeIssue(t, call(t, h.UpdateIssueFn, map[string]any{"index": idx, "clear_due_date": true, "verbose": true}))
	if iss.Deadline != nil {
		t.Fatalf("clear: %v", iss.Deadline)
	}
	if msg := callErr(t, h.UpdateIssueFn, map[string]any{"index": idx, "due_date": "2030-01-02T03:04:05Z", "clear_due_date": true}); !strings.Contains(msg, "cannot set") {
		t.Fatalf("conflict error = %q", msg)
	}
	if msg := callErr(t, h.UpdateIssueFn, map[string]any{"index": idx, "due_date": "tomorrow"}); !strings.Contains(msg, "RFC3339") {
		t.Fatalf("invalid error = %q", msg)
	}
}

func TestIssueStateChange(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := float64(openIssue(t, f, "t"))
	iss := decodeIssue(t, call(t, h.IssueStateChangeFn, map[string]any{"index": idx, "state": "closed", "verbose": true}))
	if iss.State != forgejo_sdk.StateClosed {
		t.Fatalf("state %q", iss.State)
	}
	iss = decodeIssue(t, call(t, h.IssueStateChangeFn, map[string]any{"index": idx, "state": "open", "verbose": true}))
	if iss.State != forgejo_sdk.StateOpen {
		t.Fatalf("state %q", iss.State)
	}
	if msg := callErr(t, h.IssueStateChangeFn, map[string]any{"index": idx, "state": "bogus"}); !strings.Contains(msg, "invalid state") {
		t.Fatalf("error = %q", msg)
	}
}

func TestListRepoIssues(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	a := openIssue(t, f, "a")
	b := openIssue(t, f, "b")
	call(t, h.IssueStateChangeFn, map[string]any{"index": float64(a), "state": "closed"})

	var open searchEnvelope
	if err := unmarshalResult(call(t, h.ListRepoIssuesFn, nil), &open); err != nil {
		t.Fatal(err)
	}
	if len(open.Issues) != 1 || open.Issues[0].Index != b {
		t.Fatalf("default open: %+v", open)
	}
	var all searchEnvelope
	if err := unmarshalResult(call(t, h.ListRepoIssuesFn, map[string]any{"state": "all", "sort": "oldest"}), &all); err != nil {
		t.Fatal(err)
	}
	if len(all.Issues) != 2 || all.Issues[0].Index != a {
		t.Fatalf("all/oldest: %+v", all)
	}
}

func TestListRepoIssues_KnownCeilingRefusesOverLimit(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	f.SetMaxResponseItems(2)
	for i := 0; i < 3; i++ {
		openIssue(t, f, "x")
	}
	if msg := callErr(t, h.ListRepoIssuesFn, map[string]any{"owner": testOwner, "repo": testRepo, "limit": float64(3)}); !strings.Contains(msg, "exceeds this instance's maximum of 2") {
		t.Fatalf("error = %q", msg)
	}
	var at searchEnvelope
	if err := unmarshalResult(call(t, h.ListRepoIssuesFn, map[string]any{"owner": testOwner, "repo": testRepo, "limit": float64(2)}), &at); err != nil {
		t.Fatal(err)
	}
	if len(at.Issues) != 2 {
		t.Fatalf("at ceiling returned %d, want 2", len(at.Issues))
	}
}

func TestListRepoIssues_SortNearDueDate(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	late := time.Date(2031, 1, 1, 0, 0, 0, 0, time.UTC)
	soon := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	_, _ = f.CreateIssue(context.Background(), testOwner, testRepo, forgejo_sdk.CreateIssueOption{Title: "late", Deadline: &late})
	_, _ = f.CreateIssue(context.Background(), testOwner, testRepo, forgejo_sdk.CreateIssueOption{Title: "soon", Deadline: &soon})
	var got searchEnvelope
	if err := unmarshalResult(call(t, h.ListRepoIssuesFn, map[string]any{"sort": "nearduedate"}), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Issues) != 2 || got.Issues[0].Title != "soon" {
		t.Fatalf("got %+v", got)
	}
}

func TestListRepoIssues_Validation(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	if msg := callErr(t, h.ListRepoIssuesFn, map[string]any{"repo": ""}); !strings.Contains(msg, "search_issues") {
		t.Fatalf("empty repo should point at search_issues: %q", msg)
	}
	if msg := callErr(t, h.ListRepoIssuesFn, map[string]any{"owner": ""}); !strings.Contains(msg, "owner is required") {
		t.Fatalf("empty owner: %q", msg)
	}
	if msg := callErr(t, h.ListRepoIssuesFn, map[string]any{"sort": "sideways"}); !strings.Contains(msg, "invalid sort") {
		t.Fatalf("invalid sort: %q", msg)
	}
	// A missing repository is an error with and without sort, never an empty list.
	for _, sort := range []string{"", "latest"} {
		args := map[string]any{"repo": "missing"}
		if sort != "" {
			args["sort"] = sort
		}
		if msg := callErr(t, h.ListRepoIssuesFn, args); !strings.Contains(msg, "get issues list err") {
			t.Fatalf("sort %q: %q", sort, msg)
		}
	}
}

type searchEnvelope struct {
	Issues     []forgejo_sdk.Issue `json:"issues"`
	Page       int                 `json:"page"`
	Limit      int                 `json:"limit"`
	Count      int                 `json:"count"`
	HasNext    bool                `json:"has_next"`
	TotalCount *int                `json:"total_count"`
}

func search(t *testing.T, h handlers, args map[string]any) searchEnvelope {
	t.Helper()
	res, err := h.SearchIssuesFn(context.Background(), makeReq(args))
	if err != nil || res.IsError {
		t.Fatalf("search failed: %v %+v", err, res)
	}
	var env searchEnvelope
	if err := unmarshalResult(textOf(res), &env); err != nil {
		t.Fatal(err)
	}
	return env
}

func TestSearchIssues_EnvelopeAndPaging(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	for i := 0; i < 5; i++ {
		openIssue(t, f, "needle")
	}
	// Ceiling unknown: has_next comes from a same-limit probe of the next page.
	env := search(t, h, map[string]any{"owner": testOwner, "q": "needle", "limit": float64(2)})
	if env.Count != 2 || env.Limit != 2 || env.Page != 1 || !env.HasNext || env.TotalCount == nil || *env.TotalCount != 5 {
		t.Fatalf("page 1: %+v", env)
	}
	env = search(t, h, map[string]any{"owner": testOwner, "q": "needle", "limit": float64(2), "page": float64(3)})
	if env.Count != 1 || env.HasNext {
		t.Fatalf("last page: %+v", env)
	}
	env = search(t, h, map[string]any{"owner": testOwner, "q": "absent"})
	if env.Count != 0 || env.HasNext || len(env.Issues) != 0 {
		t.Fatalf("empty: %+v", env)
	}
}

func TestSearchIssues_KnownCeiling(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	f.SetMaxResponseItems(2)
	for i := 0; i < 3; i++ {
		openIssue(t, f, "x")
	}
	if msg := callErr(t, h.SearchIssuesFn, map[string]any{"owner": testOwner, "limit": float64(3)}); !strings.Contains(msg, "exceeds this instance's maximum of 2") {
		t.Fatalf("error = %q", msg)
	}
	env := search(t, h, map[string]any{"owner": testOwner, "limit": float64(2)})
	if env.Limit != 2 || env.Count != 2 || !env.HasNext {
		t.Fatalf("at ceiling: %+v", env)
	}
	env = search(t, h, map[string]any{"owner": testOwner, "limit": float64(2), "page": float64(2)})
	if env.Count != 1 || env.HasNext {
		t.Fatalf("short page: %+v", env)
	}
}

func TestSearchIssues_MissingOwner(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	if msg := callErrNoRepo(t, h.SearchIssuesFn); !strings.Contains(msg, "owner is required") {
		t.Fatalf("error = %q", msg)
	}
}

func TestIssueComments(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := float64(openIssue(t, f, "t"))

	created := call(t, h.CreateIssueCommentFn, map[string]any{"index": idx, "body": "first", "verbose": true})
	var c forgejo_sdk.Comment
	if err := unmarshalResult(created, &c); err != nil || c.Body != "first" {
		t.Fatalf("create: %v %s", err, created)
	}
	id := float64(c.ID)

	var list commentsEnvelope
	if err := unmarshalResult(call(t, h.ListIssueCommentsFn, map[string]any{"index": idx}), &list); err != nil || len(list.Comments) != 1 {
		t.Fatalf("list: %v %+v", err, list)
	}
	if !strings.Contains(call(t, h.GetIssueCommentFn, map[string]any{"comment_id": id}), "first") {
		t.Fatal("get lost the body")
	}
	if !strings.Contains(call(t, h.EditIssueCommentFn, map[string]any{"comment_id": id, "body": "second", "verbose": true}), "second") {
		t.Fatal("edit lost the body")
	}
	if got := call(t, h.DeleteIssueCommentFn, map[string]any{"comment_id": id}); got != `{"Result":"Delete comment success"}` {
		t.Fatalf("delete: %q", got)
	}
	if msg := callErr(t, h.GetIssueCommentFn, map[string]any{"comment_id": id}); !strings.Contains(msg, "get issue comment err") {
		t.Fatalf("after delete: %q", msg)
	}
}

func TestListIssueComments_SinceBefore(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := openIssue(t, f, "t")
	comment(t, f, idx, "now")
	if msg := callErr(t, h.ListIssueCommentsFn, map[string]any{"index": float64(idx), "since": "yesterday"}); !strings.Contains(msg, "invalid since") {
		t.Fatalf("since: %q", msg)
	}
	if msg := callErr(t, h.ListIssueCommentsFn, map[string]any{"index": float64(idx), "before": "tomorrow"}); !strings.Contains(msg, "invalid before") {
		t.Fatalf("before: %q", msg)
	}
	future := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	var list commentsEnvelope
	if err := unmarshalResult(call(t, h.ListIssueCommentsFn, map[string]any{"index": float64(idx), "since": future}), &list); err != nil || len(list.Comments) != 0 {
		t.Fatalf("since future: %v %+v", err, list)
	}
}

func TestListRepoMilestones(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	f.SeedMilestone(testOwner, testRepo, "v1")
	done := f.SeedMilestone(testOwner, testRepo, "v0")
	f.CloseMilestone(testOwner, testRepo, done)
	open := call(t, h.ListRepoMilestonesFn, nil)
	if !strings.Contains(open, `"v1"`) || strings.Contains(open, `"v0"`) {
		t.Fatalf("open: %s", open)
	}
	if all := call(t, h.ListRepoMilestonesFn, map[string]any{"state": "all"}); !strings.Contains(all, `"v0"`) {
		t.Fatalf("all: %s", all)
	}
}

var compactIssueKeys = []string{"number", "url", "title", "state", "labels", "assignees", "updated_at"}

// assertCompactIssue checks the default write result: exactly the compact keys,
// and none of the body or the user object.
func assertCompactIssue(t *testing.T, text string, notContain ...string) {
	t.Helper()
	keys := mcptest.EnvelopeKeys(t, text)
	if len(keys) != len(compactIssueKeys) {
		t.Fatalf("compact result has %d keys, want %d: %s", len(keys), len(compactIssueKeys), text)
	}
	for _, k := range compactIssueKeys {
		if _, ok := keys[k]; !ok {
			t.Fatalf("compact result misses %q: %s", k, text)
		}
	}
	for _, s := range notContain {
		if strings.Contains(text, s) {
			t.Fatalf("compact result contains %q: %s", s, text)
		}
	}
}

func TestWriteResults_CompactByDefault(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := float64(openIssue(t, f, "t"))

	created := call(t, h.CreateIssueFn, map[string]any{"title": "fresh", "body": "CREATE-BODY", "assignees": "alice"})
	assertCompactIssue(t, created, "CREATE-BODY", "@")

	updated := call(t, h.UpdateIssueFn, map[string]any{"index": idx, "title": "renamed", "body": "UPDATE-BODY"})
	assertCompactIssue(t, updated, "UPDATE-BODY", "@")

	closed := call(t, h.IssueStateChangeFn, map[string]any{"index": idx, "state": "closed"})
	assertCompactIssue(t, closed, "@")
	if !strings.Contains(closed, `"state":"closed"`) || !strings.Contains(updated, `"title":"renamed"`) {
		t.Fatalf("compact fields wrong: updated=%s closed=%s", updated, closed)
	}
}

func TestWriteResults_CompactCarriesLabelAndAssigneeNames(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := float64(openIssue(t, f, "t"))
	text := call(t, h.UpdateIssueFn, map[string]any{"index": idx, "assignees": "alice, bob"})
	keys := mcptest.EnvelopeKeys(t, text)
	var assignees []string
	if err := json.Unmarshal(keys["assignees"], &assignees); err != nil {
		t.Fatalf("assignees not a string list: %v (%s)", err, text)
	}
	if strings.Join(assignees, ",") != "alice,bob" {
		t.Fatalf("assignees = %v", assignees)
	}
}

func TestWriteResults_VerboseReturnsFullEntity(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := float64(openIssue(t, f, "t"))
	for name, text := range map[string]string{
		"create": call(t, h.CreateIssueFn, map[string]any{"title": "fresh", "body": "VERBOSE-BODY", "verbose": true}),
		"update": call(t, h.UpdateIssueFn, map[string]any{"index": idx, "body": "VERBOSE-BODY", "verbose": true}),
		"state":  call(t, h.IssueStateChangeFn, map[string]any{"index": idx, "state": "closed", "verbose": true}),
	} {
		keys := mcptest.EnvelopeKeys(t, text)
		if _, ok := keys["body"]; !ok {
			t.Errorf("%s verbose: no body key: %s", name, text)
		}
		if _, ok := keys["user"]; !ok {
			t.Errorf("%s verbose: no user key: %s", name, text)
		}
		if name != "state" && !strings.Contains(text, "VERBOSE-BODY") {
			t.Errorf("%s verbose: body not returned: %s", name, text)
		}
	}
}

func TestWriteResults_VerboseFalseIsCompact(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	text := call(t, h.CreateIssueFn, map[string]any{"title": "fresh", "verbose": false})
	assertCompactIssue(t, text)
}

// commentsEnvelope is the list_issue_comments response: the comments of one
// page with the continuation signal.
type commentsEnvelope struct {
	Comments []forgejo_sdk.Comment `json:"comments"`
	Page     int                   `json:"page"`
	Limit    int                   `json:"limit"`
	Count    int                   `json:"count"`
	HasNext  bool                  `json:"has_next"`
}
