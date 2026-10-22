// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"encoding/json"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// result unwraps the {"Result": ...} envelope that a tool result carries and
// returns the JSON object inside it.
func result(t *testing.T, text string) map[string]any {
	t.Helper()
	var env struct {
		Result map[string]any `json:"Result"`
	}
	if err := json.Unmarshal([]byte(text), &env); err != nil || env.Result == nil {
		t.Fatalf("decoding %s: %v", text, err)
	}
	return env.Result
}

// issueJSON decodes a compact read: one object.
func issueJSON(t *testing.T, text string) map[string]any {
	t.Helper()
	return result(t, text)
}

// listedIssues decodes a compact list page and returns its issues.
func listedIssues(t *testing.T, text string) []map[string]any {
	t.Helper()
	issues, ok := result(t, text)["issues"].([]any)
	if !ok {
		t.Fatalf("no issues list in %s", text)
	}
	out := make([]map[string]any, 0, len(issues))
	for _, v := range issues {
		out = append(out, v.(map[string]any))
	}
	return out
}

func seedIssueWithMilestone(t *testing.T, f *fake.Forgejo, title string, ms int64) int64 {
	t.Helper()
	iss, err := f.CreateIssue(context.Background(), testOwner, testRepo, forgejo_sdk.CreateIssueOption{Title: title, Milestone: ms})
	if err != nil {
		t.Fatalf("seeding issue: %v", err)
	}
	return iss.Index
}

func TestGetIssueByIndex_CompactCarriesMilestone(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	ms := f.SeedMilestone(testOwner, testRepo, "v1")
	idx := seedIssueWithMilestone(t, f, "in a milestone", ms)

	got := issueJSON(t, call(t, h.GetIssueByIndexFn, map[string]any{"index": float64(idx)}))
	m, ok := got["milestone"].(map[string]any)
	if !ok {
		t.Fatalf("milestone = %v, want {id, title}", got["milestone"])
	}
	if m["id"] != float64(ms) || m["title"] != "v1" || len(m) != 2 {
		t.Fatalf("milestone = %v", m)
	}
	if got["is_pull_request"] != false {
		t.Fatalf("is_pull_request = %v, want false", got["is_pull_request"])
	}
	if _, ok := got["body"]; !ok {
		t.Fatal("compact read keeps body")
	}
}

func TestGetIssueByIndex_NoMilestoneIsNull(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := openIssue(t, f, "no milestone")

	got := issueJSON(t, call(t, h.GetIssueByIndexFn, map[string]any{"index": float64(idx)}))
	if v, ok := got["milestone"]; !ok || v != nil {
		t.Fatalf("milestone = %v (present=%v), want null", v, ok)
	}
}

func TestGetIssueByIndex_FlagsAPullRequestIndex(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := openIssue(t, f, "really a pull request")
	f.MarkPullRequest(testOwner, testRepo, idx)

	got := issueJSON(t, call(t, h.GetIssueByIndexFn, map[string]any{"index": float64(idx)}))
	if got["is_pull_request"] != true {
		t.Fatalf("is_pull_request = %v, want true", got["is_pull_request"])
	}
}

func TestListRepoIssues_MarksPullRequests(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	plain := openIssue(t, f, "plain")
	pr := openIssue(t, f, "pull")
	f.MarkPullRequest(testOwner, testRepo, pr)

	byIndex := map[float64]map[string]any{}
	for _, iss := range listedIssues(t, call(t, h.ListRepoIssuesFn, nil)) {
		byIndex[iss["number"].(float64)] = iss
	}
	if byIndex[float64(plain)]["is_pull_request"] != false {
		t.Fatalf("plain issue flag = %v", byIndex[float64(plain)]["is_pull_request"])
	}
	if byIndex[float64(pr)]["is_pull_request"] != true {
		t.Fatalf("pull request flag = %v", byIndex[float64(pr)]["is_pull_request"])
	}
	if _, ok := byIndex[float64(plain)]["milestone"]; !ok {
		t.Fatal("listed issue lacks milestone key")
	}
}

func TestListIssueDependencies_IsCompactWithBothFields(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	a := openIssue(t, f, "dependent")
	b := seedIssueWithMilestone(t, f, "blocker", f.SeedMilestone(testOwner, testRepo, "v2"))
	if err := f.AddIssueDependency(context.Background(), testOwner, testRepo, a, forge.IssueRef{Owner: testOwner, Repo: testRepo, Index: b}); err != nil {
		t.Fatal(err)
	}

	issues := listedIssues(t, call(t, h.ListIssueDependenciesFn, map[string]any{"index": float64(a)}))
	if len(issues) != 1 {
		t.Fatalf("dependencies = %d, want 1", len(issues))
	}
	dep := issues[0]
	if _, ok := dep["body"]; ok {
		t.Fatal("dependency entries must be compact, not carry body")
	}
	if dep["number"] != float64(b) || dep["is_pull_request"] != false {
		t.Fatalf("dependency = %v", dep)
	}
	if m, ok := dep["milestone"].(map[string]any); !ok || m["title"] != "v2" {
		t.Fatalf("dependency milestone = %v", dep["milestone"])
	}
}
