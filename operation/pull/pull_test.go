// SPDX-License-Identifier: GPL-3.0-or-later

package pull

import (
	"context"
	"errors"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/fake"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

const multiFileDiff = `diff --git a/cmd/main.go b/cmd/main.go
index 1234..5678 100644
--- a/cmd/main.go
+++ b/cmd/main.go
@@ -1,3 +1,4 @@
 package main

+import "fmt"
 func main() {}
diff --git a/README.md b/README.md
index abcd..ef01 100644
--- a/README.md
+++ b/README.md
@@ -10,2 +10,3 @@
 line ten
+line eleven
 line twelve`

func diffFixture(t *testing.T) (handlers, int64) {
	t.Helper()
	h, f := newFixture(t)
	idx := openPR(t, f, "diffed")
	if err := f.SetPullRequestDiff(testOwner, testRepo, idx, multiFileDiff); err != nil {
		t.Fatal(err)
	}
	return h, idx
}

func TestGetPullRequestDiff_NoFilePathReturnsFullDiff(t *testing.T) {
	t.Parallel()
	h, idx := diffFixture(t)
	if got := call(t, h.GetPullRequestDiffFn, map[string]any{"index": float64(idx)}); got != multiFileDiff {
		t.Fatalf("expected full diff back, got:\n%s", got)
	}
}

func TestGetPullRequestDiff_FilePathReturnsSlice(t *testing.T) {
	t.Parallel()
	h, idx := diffFixture(t)
	out := call(t, h.GetPullRequestDiffFn, map[string]any{"index": float64(idx), "file_path": "cmd/main.go"})
	if !strings.Contains(out, "diff --git a/cmd/main.go b/cmd/main.go") {
		t.Fatalf("expected slice header, got:\n%s", out)
	}
	if strings.Contains(out, "README.md") {
		t.Fatalf("slice should not contain other files:\n%s", out)
	}
}

func TestGetPullRequestDiff_FilePathNotFound(t *testing.T) {
	t.Parallel()
	h, idx := diffFixture(t)
	_, err := h.GetPullRequestDiffFn(context.Background(), makeReq(map[string]any{
		"index": float64(idx), "file_path": "does/not/exist.go",
	}))
	if err == nil {
		t.Fatal("expected error for missing file_path, got nil")
	}
	if !strings.Contains(err.Error(), "does/not/exist.go") {
		t.Fatalf("expected error to name the missing path, got %v", err)
	}
}

func TestGetPullRequestByIndex(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := openPR(t, f, "feat: something")
	if out := call(t, h.GetPullRequestByIndexFn, map[string]any{"index": float64(idx)}); !strings.Contains(out, "feat: something") {
		t.Fatalf("result does not carry the pull request: %s", out)
	}
	if _, err := h.GetPullRequestByIndexFn(context.Background(), makeReq(map[string]any{"index": float64(99)})); err == nil {
		t.Fatal("an unknown pull request must be an error")
	}
}

func TestListRepoPullRequests_DefaultsToOpenAndPaginates(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	merged := openPR(t, f, "already merged")
	openPR(t, f, "open one")
	openPR(t, f, "open two")
	if _, err := f.MergePullRequest(context.Background(), testOwner, testRepo, merged, forgejo_sdk.MergePullRequestOption{}); err != nil {
		t.Fatal(err)
	}

	out := call(t, h.ListRepoPullRequestsFn, nil)
	if strings.Contains(out, "already merged") || !strings.Contains(out, "open one") || !strings.Contains(out, "open two") {
		t.Fatalf("default list should be the open ones: %s", out)
	}
	if out := call(t, h.ListRepoPullRequestsFn, map[string]any{"limit": float64(1)}); strings.Contains(out, "open one") || !strings.Contains(out, "open two") {
		t.Fatalf("limit 1 should return only the newest: %s", out)
	}
	if out := call(t, h.ListRepoPullRequestsFn, map[string]any{"state": "all"}); !strings.Contains(out, "already merged") {
		t.Fatalf("state=all should include merged: %s", out)
	}
}

func TestListRepoPullRequests_InvalidMilestone(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	if _, err := h.ListRepoPullRequestsFn(context.Background(), makeReq(map[string]any{"milestone": "soon"})); err == nil {
		t.Fatal("a non-numeric milestone must be an error")
	}
}

func TestCreateThenUpdatePullRequest(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	out := call(t, h.CreatePullRequestFn, map[string]any{"head": "feat", "base": "main", "title": "first title"})
	if !strings.Contains(out, "first title") {
		t.Fatalf("create result: %s", out)
	}
	out = call(t, h.UpdatePullRequestFn, map[string]any{"index": float64(1), "title": "second title"})
	if !strings.Contains(out, "second title") {
		t.Fatalf("update result: %s", out)
	}
	if got := call(t, h.GetPullRequestByIndexFn, map[string]any{"index": float64(1)}); !strings.Contains(got, "second title") {
		t.Fatalf("update was not kept: %s", got)
	}
}

func TestMergePullRequest(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := openPR(t, f, "to merge")

	if got := call(t, h.MergePullRequestFn, map[string]any{"index": float64(idx), "style": "merge"}); got != "Pull request merged successfully" {
		t.Fatalf("got %q", got)
	}
	// A second merge is declined by Forgejo; the tool reports the status.
	_, err := h.MergePullRequestFn(context.Background(), makeReq(map[string]any{"index": float64(idx), "style": "merge"}))
	if err == nil || !strings.Contains(err.Error(), "HTTP 405") {
		t.Fatalf("want the declined status reported, got %v", err)
	}
}

func TestMergePullRequest_WhenChecksSucceed(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := openPR(t, f, "scheduled")
	got := call(t, h.MergePullRequestFn, map[string]any{"index": float64(idx), "style": "merge", "merge_when_checks_succeed": true})
	if got != "Pull request scheduled to merge when all checks succeed" {
		t.Fatalf("got %q", got)
	}
}

func TestListPullRequestFiles(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := openPR(t, f, "files")
	if err := f.SetPullRequestFiles(testOwner, testRepo, idx,
		&forgejo_sdk.ChangedFile{Filename: "a.go"}, &forgejo_sdk.ChangedFile{Filename: "b.go"}); err != nil {
		t.Fatal(err)
	}
	out := call(t, h.ListPullRequestFilesFn, map[string]any{"index": float64(idx)})
	if !strings.Contains(out, "a.go") || !strings.Contains(out, "b.go") {
		t.Fatalf("got %s", out)
	}
}

func TestToolsFailWhenFactoryRefuses(t *testing.T) {
	t.Parallel()
	refusal := errors.New("no credential for this request")
	h := handlers{forge: fake.Factory{Err: refusal}}
	_, err := h.GetPullRequestByIndexFn(context.Background(), makeReq(map[string]any{"index": float64(1)}))
	if !errors.Is(err, refusal) {
		t.Fatalf("got %v, want the factory's refusal", err)
	}
}
