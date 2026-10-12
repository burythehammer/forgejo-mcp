// SPDX-License-Identifier: GPL-3.0-or-later

package compact

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
	"github.com/mark3labs/mcp-go/mcp"
)

var updated = time.Date(2026, 10, 1, 9, 30, 0, 0, time.UTC)

func sampleIssue() *model.Issue {
	return &model.Issue{
		Index:   7,
		URL:     "https://forge.example/api/v1/repos/o/r/issues/7",
		HTMLURL: "https://forge.example/o/r/issues/7",
		Title:   "Compact me",
		Body:    "a very long body that must not be echoed back",
		State:   "open",
		Poster:  &model.User{UserName: "matt", Email: "matt@example.test", FullName: "Matt"},
		Labels:  []*model.Label{{Name: "bug"}, {Name: "ui"}},
		Assignees: []*model.User{
			{UserName: "alice", Email: "alice@example.test"},
			{UserName: "bob"},
		},
		Updated: updated,
	}
}

func sampleTextOf(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if res == nil || len(res.Content) != 1 {
		t.Fatalf("unexpected result %+v", res)
	}
	tc, ok := res.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("content is %T, not text", res.Content[0])
	}
	return tc.Text
}

// keysOf decodes the {"Result": ...} envelope and returns the object's keys.
func keysOf(t *testing.T, text string) map[string]json.RawMessage {
	t.Helper()
	var wrapped struct{ Result map[string]json.RawMessage }
	if err := json.Unmarshal([]byte(text), &wrapped); err != nil {
		t.Fatalf("not a result envelope: %v\n%s", err, text)
	}
	return wrapped.Result
}

func TestOfIssue_CompactFieldsOnly(t *testing.T) {
	t.Parallel()
	b, err := json.Marshal(OfIssue(sampleIssue()))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"number", "url", "title", "state", "labels", "assignees", "updated_at"} {
		if _, ok := got[k]; !ok {
			t.Errorf("missing key %q in %s", k, b)
		}
	}
	for _, k := range []string{"body", "user", "email", "html_url", "id"} {
		if _, ok := got[k]; ok {
			t.Errorf("compact issue must not carry %q: %s", k, b)
		}
	}
	if strings.Contains(string(b), "very long body") || strings.Contains(string(b), "matt@example.test") {
		t.Fatalf("body or user object leaked: %s", b)
	}
	if got["number"] != float64(7) || got["url"] != "https://forge.example/o/r/issues/7" || got["title"] != "Compact me" || got["state"] != "open" {
		t.Fatalf("identifying fields wrong: %s", b)
	}
	if labels := got["labels"].([]any); len(labels) != 2 || labels[0] != "bug" || labels[1] != "ui" {
		t.Fatalf("labels = %v", labels)
	}
	if assignees := got["assignees"].([]any); len(assignees) != 2 || assignees[0] != "alice" || assignees[1] != "bob" {
		t.Fatalf("assignees = %v", assignees)
	}
}

func TestOfIssue_EmptyLabelsAndAssigneesAreArrays(t *testing.T) {
	t.Parallel()
	b, err := json.Marshal(OfIssue(&model.Issue{Index: 1}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"labels":[]`) || !strings.Contains(string(b), `"assignees":[]`) {
		t.Fatalf("want empty arrays, got %s", b)
	}
}

func TestOfPullRequest_AddsHeadAndBase(t *testing.T) {
	t.Parallel()
	pr := &model.PullRequest{
		Index:    3,
		HTMLURL:  "https://forge.example/o/r/pulls/3",
		Title:    "PR",
		Body:     "pr body",
		State:    "open",
		Labels:   []*model.Label{{Name: "ready"}},
		Assignee: &model.User{UserName: "carol"},
		Assignees: []*model.User{
			{UserName: "carol"},
		},
		Head:    &model.PRBranchInfo{Name: "feature"},
		Base:    &model.PRBranchInfo{Name: "main"},
		Updated: &updated,
	}
	b, err := json.Marshal(OfPullRequest(pr))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{`"number":3`, `"head":"feature"`, `"base":"main"`, `"labels":["ready"]`, `"assignees":["carol"]`, `"state":"open"`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s in %s", want, s)
		}
	}
	if strings.Contains(s, "pr body") {
		t.Fatalf("body leaked: %s", s)
	}
}

func TestRenderIssue_DefaultIsCompactVerboseIsFull(t *testing.T) {
	t.Parallel()
	res, err := RenderIssue(sampleIssue(), nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := keysOf(t, sampleTextOf(t, res)); got["body"] != nil || got["user"] != nil {
		t.Fatalf("default result carries body or user: %v", got)
	}

	res, err = RenderIssue(sampleIssue(), nil, true)
	if err != nil {
		t.Fatal(err)
	}
	got := keysOf(t, sampleTextOf(t, res))
	if got["body"] == nil || got["user"] == nil {
		t.Fatalf("verbose result must be the full entity, keys: %v", got)
	}
}

func TestRenderIssue_ErrorIsPassedThrough(t *testing.T) {
	t.Parallel()
	res, err := RenderIssue(nil, errTest, false)
	if err == nil && (res == nil || !res.IsError) {
		t.Fatalf("expected an error, got res=%+v err=%v", res, err)
	}
}

func TestRenderPullRequest_VerboseIsFull(t *testing.T) {
	t.Parallel()
	pr := &model.PullRequest{Index: 1, Body: "pr body", Head: &model.PRBranchInfo{Name: "h"}, Base: &model.PRBranchInfo{Name: "b"}}
	res, err := RenderPullRequest(pr, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sampleTextOf(t, res), "pr body") {
		t.Fatalf("verbose PR lost its body")
	}
	res, err = RenderPullRequest(pr, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if s := sampleTextOf(t, res); strings.Contains(s, "pr body") || !strings.Contains(s, `"head":"h"`) {
		t.Fatalf("compact PR wrong: %s", s)
	}
}

type testError string

func (e testError) Error() string { return string(e) }

const errTest = testError("boom")

// The verbose description is shared by issue, pull request and comment writes,
// so it must not name the fields of one entity kind.
func TestVerboseDescription_IsGeneric(t *testing.T) {
	t.Parallel()
	const want = "Return the full entity instead of the compact summary. Default false."
	if VerboseDescription != want {
		t.Fatalf("VerboseDescription = %q, want %q", VerboseDescription, want)
	}
}
