// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcptest"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/outfile"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// outFileTempDir points the temp dir at a fresh directory and returns the
// directory the module writes into, which must not exist until a write.
func outFileTempDir(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	return filepath.Join(tmp, "forgejo-mcp")
}

// seedOpen opens n issues in testOwner/testRepo and returns their indexes.
func seedOpen(t *testing.T, f *fake.Forgejo, n int) []int64 {
	t.Helper()
	var idx []int64
	for i := 0; i < n; i++ {
		idx = append(idx, openIssue(t, f, "issue"))
	}
	return idx
}

// outFileReply is the reply of list_repo_issues with outFile.
type outFileReply struct {
	Path  string  `json:"path"`
	Count int     `json:"count"`
	Keys  []int64 `json:"keys"`
}

// fileNumbers reads the file at path and returns the issue number of each row
// in file order.
func fileNumbers(t *testing.T, path string) []int64 {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var rows []struct {
		Number int64 `json:"number"`
	}
	if err := json.Unmarshal(b, &rows); err != nil {
		t.Fatalf("file is not a JSON array of rows: %v", err)
	}
	out := make([]int64, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Number)
	}
	return out
}

func noFileWritten(t *testing.T, dir string) {
	t.Helper()
	if entries, err := os.ReadDir(dir); err == nil && len(entries) != 0 {
		t.Fatalf("a refused call wrote %v under %s", entries, dir)
	}
}

func TestListRepoIssuesOutFile_WritesWholeListing(t *testing.T) {
	dir := outFileTempDir(t)
	h, f := newFixture(t)
	want := seedOpen(t, f, 45) // three pages at the default limit of 20

	var rep outFileReply
	if err := unmarshalResult(call(t, h.ListRepoIssuesFn, map[string]any{"outFile": "all.json"}), &rep); err != nil {
		t.Fatal(err)
	}
	if rep.Path != filepath.Join(dir, "all.json") {
		t.Fatalf("path = %q, want %q", rep.Path, filepath.Join(dir, "all.json"))
	}
	if rep.Count != 45 || len(rep.Keys) != 45 {
		t.Fatalf("reply count %d, keys %d; want 45 each", rep.Count, len(rep.Keys))
	}
	got := fileNumbers(t, rep.Path)
	if !reflect.DeepEqual(got, rep.Keys) {
		t.Fatalf("file numbers %v differ from reply keys %v", got, rep.Keys)
	}
	seen := map[int64]bool{}
	for _, k := range got {
		seen[k] = true
	}
	for _, w := range want {
		if !seen[w] {
			t.Fatalf("issue %d missing from the file", w)
		}
	}
}

func TestListRepoIssuesOutFile_BareNameOnly(t *testing.T) {
	dir := outFileTempDir(t)
	h, f := newFixture(t)
	seedOpen(t, f, 2)
	for _, bad := range []string{"a/b", "..", "../x", "/abs/x.json", ""} {
		msg := callErr(t, h.ListRepoIssuesFn, map[string]any{"outFile": bad})
		if !strings.Contains(msg, "outFile") {
			t.Errorf("outFile %q: error %q does not name outFile", bad, msg)
		}
	}
	noFileWritten(t, dir)
}

func TestListRepoIssuesOutFile_RefusesPageAndLimit(t *testing.T) {
	dir := outFileTempDir(t)
	h, f := newFixture(t)
	seedOpen(t, f, 2)
	if msg := callErr(t, h.ListRepoIssuesFn, map[string]any{"outFile": "p.json", "page": float64(2)}); !strings.Contains(msg, "page") {
		t.Fatalf("page with outFile: error %q", msg)
	}
	if msg := callErr(t, h.ListRepoIssuesFn, map[string]any{"outFile": "p.json", "limit": float64(5)}); !strings.Contains(msg, "limit") {
		t.Fatalf("limit with outFile: error %q", msg)
	}
	noFileWritten(t, dir)
}

func TestListRepoIssuesOutFile_RefusedOnNetworkTransport(t *testing.T) {
	dir := outFileTempDir(t)
	h, f := newFixture(t)
	seedOpen(t, f, 2)
	ctx := outfile.MarkRemote(context.Background())
	res, err := h.ListRepoIssuesFn(ctx, repoReq(map[string]any{"outFile": "net.json"}))
	if err != nil {
		t.Fatal(err)
	}
	if res == nil || !res.IsError || !strings.Contains(textOf(res), "stdio") {
		t.Fatalf("network transport: result %+v, want a refusal naming stdio", res)
	}
	noFileWritten(t, dir)
}

func TestListRepoIssuesOutFile_OverwritesExisting(t *testing.T) {
	outFileTempDir(t)
	h, f := newFixture(t)
	seedOpen(t, f, 3)
	call(t, h.ListRepoIssuesFn, map[string]any{"outFile": "same.json"})

	var rep outFileReply
	if err := unmarshalResult(call(t, h.ListRepoIssuesFn, map[string]any{"outFile": "same.json", "state": "closed"}), &rep); err != nil {
		t.Fatal(err)
	}
	if rep.Count != 0 || len(fileNumbers(t, rep.Path)) != 0 {
		t.Fatalf("overwrite with no closed issues: reply %+v", rep)
	}
}

// pagedStub serves n fixed issues in pages of the requested size, so a file
// golden covers a listing that spans more than one page with stable bytes.
type pagedStub struct {
	forge.Forgejo
	n int
}

func (s *pagedStub) MaxResponseItems(context.Context) (int, bool) { return 0, false }

func (s *pagedStub) ListRepoIssues(_ context.Context, _, _ string, opt forge.ListIssuesOptions) ([]*forgejo_sdk.Issue, forge.PageInfo, error) {
	page, size := opt.Page, opt.PageSize
	if page < 1 {
		page = 1
	}
	var out []*forgejo_sdk.Issue
	for i := (page - 1) * size; i < page*size && i < s.n; i++ {
		iss := goldenIssue("row")
		iss.Index = int64(i + 1)
		out = append(out, iss)
	}
	return out, forge.PageInfo{}, nil
}

func TestListRepoIssuesOutFile_Golden(t *testing.T) {
	dir := outFileTempDir(t)
	h := withPort(&pagedStub{n: 25})
	res, err := h.ListRepoIssuesFn(context.Background(), repoReq(map[string]any{"outFile": "golden.json"}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("golden call failed: err=%v res=%+v", err, res)
	}

	var rep outFileReply
	if err := json.Unmarshal([]byte(textOf(res)), &rep); err != nil {
		t.Fatal(err)
	}
	if rep.Path != filepath.Join(dir, "golden.json") {
		t.Fatalf("path = %q", rep.Path)
	}
	// The temp dir is random per run, so the golden holds a placeholder.
	rep.Path = "<tmp>/forgejo-mcp/golden.json"
	reply, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	mcptest.Golden(t, "list_repo_issues_outfile_reply", reply)

	file, err := os.ReadFile(filepath.Join(dir, "golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	mcptest.Golden(t, "list_repo_issues_outfile_file", file)
}
