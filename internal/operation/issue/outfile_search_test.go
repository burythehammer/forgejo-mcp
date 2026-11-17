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

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/list"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcptest"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

// searchOutReq is a search_issues request for testOwner; search takes no repo.
func searchOutReq(args map[string]any) mcp.CallToolRequest {
	base := map[string]any{"owner": testOwner}
	for k, v := range args {
		base[k] = v
	}
	return makeReq(base)
}

// searchOutCall runs search_issues with outFile and returns its text, failing
// the test on an error result.
func searchOutCall(t *testing.T, h handlers, args map[string]any) string {
	t.Helper()
	res, err := h.SearchIssuesFn(context.Background(), searchOutReq(args))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("search_issues failed: err=%v res=%+v", err, res)
	}
	return textOf(res)
}

// searchOutErr runs search_issues expected to refuse and returns the error text.
func searchOutErr(t *testing.T, h handlers, args map[string]any) string {
	t.Helper()
	res, err := h.SearchIssuesFn(context.Background(), searchOutReq(args))
	if err != nil {
		return err.Error()
	}
	if res == nil || !res.IsError {
		t.Fatalf("expected a refusal, got %+v", res)
	}
	return textOf(res)
}

func TestSearchIssuesOutFile_WritesWholeListing(t *testing.T) {
	dir := outFileTempDir(t)
	h, f := newFixture(t)
	want := seedOpen(t, f, 45) // three pages at the default limit of 20

	var rep outFileReply
	if err := unmarshalResult(searchOutCall(t, h, map[string]any{"outFile": "all.json"}), &rep); err != nil {
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

func TestSearchIssuesOutFile_BareNameOnly(t *testing.T) {
	dir := outFileTempDir(t)
	h, f := newFixture(t)
	seedOpen(t, f, 2)
	for _, bad := range []string{"a/b", "..", "../x", "/abs/x.json", ""} {
		msg := searchOutErr(t, h, map[string]any{"outFile": bad})
		if !strings.Contains(msg, "outFile") {
			t.Errorf("outFile %q: error %q does not name outFile", bad, msg)
		}
	}
	noFileWritten(t, dir)
}

func TestSearchIssuesOutFile_RefusesPageAndLimit(t *testing.T) {
	dir := outFileTempDir(t)
	h, f := newFixture(t)
	seedOpen(t, f, 2)
	if msg := searchOutErr(t, h, map[string]any{"outFile": "p.json", "page": float64(2)}); !strings.Contains(msg, "page") {
		t.Fatalf("page with outFile: error %q", msg)
	}
	if msg := searchOutErr(t, h, map[string]any{"outFile": "p.json", "limit": float64(5)}); !strings.Contains(msg, "limit") {
		t.Fatalf("limit with outFile: error %q", msg)
	}
	noFileWritten(t, dir)
}

func TestSearchIssuesOutFile_RefusedOnNetworkTransport(t *testing.T) {
	dir := outFileTempDir(t)
	h, f := newFixture(t)
	seedOpen(t, f, 2)
	ctx := forge.MarkRemote(context.Background())
	res, err := h.SearchIssuesFn(ctx, searchOutReq(map[string]any{"outFile": "net.json"}))
	var msg string
	switch {
	case err != nil:
		msg = err.Error()
	case res != nil && res.IsError:
		msg = textOf(res)
	default:
		t.Fatalf("network transport: no refusal, result %+v", res)
	}
	if !strings.Contains(msg, "stdio") {
		t.Fatalf("network transport refusal %q does not name stdio", msg)
	}
	noFileWritten(t, dir)
}

func TestSearchIssuesOutFile_OverwritesExisting(t *testing.T) {
	outFileTempDir(t)
	h, f := newFixture(t)
	seedOpen(t, f, 3)
	searchOutCall(t, h, map[string]any{"outFile": "same.json"})

	var rep outFileReply
	if err := unmarshalResult(searchOutCall(t, h, map[string]any{"outFile": "same.json", "state": "closed"}), &rep); err != nil {
		t.Fatal(err)
	}
	if rep.Count != 0 || len(fileNumbers(t, rep.Path)) != 0 {
		t.Fatalf("overwrite with no closed issues: reply %+v", rep)
	}
}

// pagedSearchStub serves n fixed issues in pages of the requested size and
// reports n as the grand total, so a file golden covers a multi-page search
// with stable bytes.
type pagedSearchStub struct {
	forge.Forgejo
	n int
}

func (s *pagedSearchStub) MaxResponseItems(context.Context) (int, bool) { return 0, false }

func (s *pagedSearchStub) SearchIssues(_ context.Context, opt forgejo_sdk.ListIssueOption) ([]*forgejo_sdk.Issue, forge.PageInfo, error) {
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
	return out, forge.PageInfo{TotalCount: s.n, TotalKnown: true}, nil
}

func TestSearchIssuesOutFile_Golden(t *testing.T) {
	dir := outFileTempDir(t)
	h := withPort(&pagedSearchStub{n: 25})
	res, err := h.SearchIssuesFn(context.Background(), searchOutReq(map[string]any{"outFile": "golden.json"}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("golden call failed: err=%v res=%+v", err, res)
	}

	var rep outFileReply
	if err := unmarshalResult(textOf(res), &rep); err != nil {
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
	mcptest.Golden(t, "search_issues_outfile_reply", reply)

	file, err := os.ReadFile(filepath.Join(dir, "golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	mcptest.Golden(t, "search_issues_outfile_file", file)
}

func TestSearchIssuesOutFile_AtTheCeilingWritesWhole(t *testing.T) {
	dir := outFileTempDir(t)
	h := withPort(&pagedSearchStub{n: list.FileCeiling})
	var rep outFileReply
	if err := unmarshalResult(searchOutCall(t, h, map[string]any{"outFile": "ceiling.json"}), &rep); err != nil {
		t.Fatal(err)
	}
	if rep.Path != filepath.Join(dir, "ceiling.json") || rep.Count != list.FileCeiling {
		t.Fatalf("reply %+v, want %d rows at %s", rep, list.FileCeiling, dir)
	}
	if got := fileNumbers(t, rep.Path); len(got) != list.FileCeiling {
		t.Fatalf("file holds %d rows, want %d", len(got), list.FileCeiling)
	}
}

func TestSearchIssuesOutFile_OneOverTheCeilingFailsAndWritesNothing(t *testing.T) {
	dir := outFileTempDir(t)
	h := withPort(&pagedSearchStub{n: list.FileCeiling + 1})
	msg := searchOutErr(t, h, map[string]any{"outFile": "over.json"})
	if !strings.Contains(msg, "1000") || !strings.Contains(msg, "ceiling") {
		t.Fatalf("error %q does not name the ceiling", msg)
	}
	noFileWritten(t, dir)
	mcptest.Golden(t, "search_issues_outfile_over_ceiling", []byte(msg+"\n"))
}

func TestSearchIssuesOutFile_OverTheCeilingKeepsAnEarlierFile(t *testing.T) {
	dir := outFileTempDir(t)
	h, f := newFixture(t)
	seedOpen(t, f, 2)
	searchOutCall(t, h, map[string]any{"outFile": "keep.json"})
	before, err := os.ReadFile(filepath.Join(dir, "keep.json"))
	if err != nil {
		t.Fatal(err)
	}

	over := withPort(&pagedSearchStub{n: list.FileCeiling + 1})
	searchOutErr(t, over, map[string]any{"outFile": "keep.json"})
	after, err := os.ReadFile(filepath.Join(dir, "keep.json"))
	if err != nil {
		t.Fatalf("the earlier file was removed by a failed run: %v", err)
	}
	if string(before) != string(after) {
		t.Fatal("a failed over-ceiling run clobbered the earlier file")
	}
}

// TestSearchIssuesOutFile_RowsCarryMilestoneAndHonourVerbose checks the row
// shape: compact rows (milestone included) by default, full issues when
// verbose is set.
func TestSearchIssuesOutFile_RowsCarryMilestoneAndHonourVerbose(t *testing.T) {
	outFileTempDir(t)
	h, f := newFixture(t)
	seedOpen(t, f, 2)

	keysOf := func(args map[string]any) map[string]json.RawMessage {
		t.Helper()
		var rep outFileReply
		if err := unmarshalResult(searchOutCall(t, h, args), &rep); err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(rep.Path)
		if err != nil {
			t.Fatal(err)
		}
		var rows []map[string]json.RawMessage
		if err := json.Unmarshal(b, &rows); err != nil || len(rows) == 0 {
			t.Fatalf("file rows: %v (%d)", err, len(rows))
		}
		return rows[0]
	}

	compactRow := keysOf(map[string]any{"outFile": "compact.json"})
	if _, ok := compactRow["milestone"]; !ok {
		t.Fatal("compact row has no milestone key")
	}
	if _, ok := compactRow["body"]; ok {
		t.Fatal("compact row carries the full body")
	}

	verboseRow := keysOf(map[string]any{"outFile": "verbose.json", "verbose": true})
	if _, ok := verboseRow["body"]; !ok {
		t.Fatal("verbose row lacks the full issue body")
	}
}
