// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcptest"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// These tests drive replace_issue_body_line through an MCP call against the
// fake forge. They assert the stored body after each call, so a refusal that
// changed anything would show.

func storedBody(t *testing.T, f *fake.Forgejo, index int64) string {
	t.Helper()
	iss, err := f.GetIssue(context.Background(), testOwner, testRepo, index)
	if err != nil {
		t.Fatalf("reading stored issue: %v", err)
	}
	return iss.Body
}

func seedBody(t *testing.T, f *fake.Forgejo, body string) int64 {
	t.Helper()
	idx := openIssue(t, f, "t")
	if _, err := f.EditIssue(context.Background(), testOwner, testRepo, idx, forgejo_sdk.EditIssueOption{Body: &body}); err != nil {
		t.Fatalf("seeding body: %v", err)
	}
	return idx
}

func TestReplaceIssueBodyLine_ReplacesOneMatch(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := seedBody(t, f, "## AC\n- [ ] first\n- [ ] second\n")

	out := call(t, h.ReplaceIssueBodyLineFn, map[string]any{
		"index": float64(idx), "old": "- [ ] second", "new": "- [x] second",
	})

	want := "## AC\n- [ ] first\n- [x] second\n"
	if got := storedBody(t, f, idx); got != want {
		t.Fatalf("stored body = %q, want %q", got, want)
	}
	keys := mcptest.EnvelopeKeys(t, out)
	for _, k := range []string{"number", "updated_at", "body_sha256", "old", "new"} {
		if _, ok := keys[k]; !ok {
			t.Fatalf("result misses %q: %s", k, out)
		}
	}
	if !strings.Contains(out, `"- [x] second"`) || !strings.Contains(out, `"- [ ] second"`) {
		t.Fatalf("result does not show the replaced text: %s", out)
	}
	if !strings.Contains(out, sha256Hex(want)) {
		t.Fatalf("result body_sha256 is not the hash of the stored body: %s", out)
	}
	if strings.Contains(out, `"body"`) {
		t.Fatalf("compact result carries the full body: %s", out)
	}
}

func TestReplaceIssueBodyLine_ResultHashesStoredBody(t *testing.T) {
	t.Parallel()
	_, f := newFixture(t)
	idx := seedBody(t, f, "line a\n")
	h := withPort(driftPort{Forgejo: f, storedBody: "STORED"})

	out := call(t, h.ReplaceIssueBodyLineFn, map[string]any{
		"index": float64(idx), "old": "line a", "new": "line b",
	})
	if !strings.Contains(out, sha256Hex("STORED")) {
		t.Fatalf("result does not hash the stored body: %s", out)
	}
	if strings.Contains(out, sha256Hex("STORED\n")) {
		t.Fatalf("result hashes something else: %s", out)
	}
}

func TestReplaceIssueBodyLine_NotFoundLeavesBodyUnchanged(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	const body = "- [ ] first\n"
	idx := seedBody(t, f, body)

	msg := callErr(t, h.ReplaceIssueBodyLineFn, map[string]any{
		"index": float64(idx), "old": "- [ ] missing", "new": "- [x] missing",
	})
	if !strings.Contains(msg, "not found") {
		t.Fatalf("zero matches should say not found: %s", msg)
	}
	if got := storedBody(t, f, idx); got != body {
		t.Fatalf("body changed on refusal: %q", got)
	}
}

func TestReplaceIssueBodyLine_AmbiguousReportsCountAndLeavesBodyUnchanged(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	const body = "- [ ] item\n- [ ] item\n- [ ] item\n"
	idx := seedBody(t, f, body)

	msg := callErr(t, h.ReplaceIssueBodyLineFn, map[string]any{
		"index": float64(idx), "old": "- [ ] item", "new": "- [x] item",
	})
	if !strings.Contains(msg, "3 times") {
		t.Fatalf("ambiguous refusal should carry the match count: %s", msg)
	}
	if got := storedBody(t, f, idx); got != body {
		t.Fatalf("body changed on refusal: %q", got)
	}
}

func TestReplaceIssueBodyLine_CountsWithoutOverlap(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	const body = "aaaa"
	idx := seedBody(t, f, body)

	msg := callErr(t, h.ReplaceIssueBodyLineFn, map[string]any{
		"index": float64(idx), "old": "aa", "new": "b",
	})
	if !strings.Contains(msg, "2 times") {
		t.Fatalf("\"aa\" in \"aaaa\" should count 2 without overlap: %s", msg)
	}
	if got := storedBody(t, f, idx); got != body {
		t.Fatalf("body changed on refusal: %q", got)
	}
}

func TestReplaceIssueBodyLine_EmptyOldRefused(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	const body = "some body\n"
	idx := seedBody(t, f, body)

	msg := callErr(t, h.ReplaceIssueBodyLineFn, map[string]any{
		"index": float64(idx), "old": "", "new": "x",
	})
	if !strings.Contains(msg, "old") || !strings.Contains(msg, "empty") {
		t.Fatalf("empty old should be refused: %s", msg)
	}
	if got := storedBody(t, f, idx); got != body {
		t.Fatalf("body changed on refusal: %q", got)
	}
}

func TestReplaceIssueBodyLine_PullRequestRefused(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	const body = "- [ ] item\n"
	idx := seedBody(t, f, body)
	f.MarkPullRequest(testOwner, testRepo, idx)

	msg := callErr(t, h.ReplaceIssueBodyLineFn, map[string]any{
		"index": float64(idx), "old": "- [ ] item", "new": "- [x] item",
	})
	if !strings.Contains(msg, "pull request") {
		t.Fatalf("pull request index should be refused: %s", msg)
	}
	if got := storedBody(t, f, idx); got != body {
		t.Fatalf("pull request body changed on refusal: %q", got)
	}
}
