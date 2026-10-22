// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// commitPage reads the commit resource at uri and returns its decoded file
// list, the files_* markers and the message flag.
type commitPage struct {
	Filenames        []string
	FilesTotal       int    `json:"files_total"`
	FilesPage        int    `json:"files_page"`
	FilesTruncated   bool   `json:"files_truncated"`
	FilesNext        string `json:"files_next"`
	MessageTruncated bool   `json:"message_truncated"`
}

func readCommitPage(t *testing.T, h handlers, uri string) (commitPage, string) {
	t.Helper()
	contents, err := h.commitResourceHandler(context.Background(), mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{URI: uri},
	})
	if err != nil {
		t.Fatalf("read %s: %v", uri, err)
	}
	var raw struct {
		Files []struct {
			Filename string `json:"filename"`
		} `json:"files"`
	}
	text := contents[0].(mcp.TextResourceContents).Text
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		t.Fatal(err)
	}
	var out commitPage
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatal(err)
	}
	for _, f := range raw.Files {
		out.Filenames = append(out.Filenames, f.Filename)
	}
	md := ""
	if len(contents) > 1 {
		md = contents[1].(mcp.TextResourceContents).Text
	}
	return out, md
}

func commitURI(sha, query string) string {
	uri := "forgejo://repo/" + filesOwner + "/" + filesRepo + "/commit/" + sha
	if query != "" {
		uri += "?" + query
	}
	return uri
}

func TestCommitResourceFilesPagedWithNoGapOrOverlap(t *testing.T) {
	t.Parallel()
	h, f := newFilesFixture(t)
	var paths []string
	for i := 0; i < 45; i++ {
		paths = append(paths, fmt.Sprintf("dir/file%02d.go", i))
	}
	if err := f.AddCommit(filesOwner, filesRepo, testSHA, "many", paths...); err != nil {
		t.Fatal(err)
	}

	first, _ := readCommitPage(t, h, commitURI(testSHA, ""))
	if !first.FilesTruncated || first.FilesTotal != 45 || first.FilesPage != 1 {
		t.Fatalf("page 1 markers: %+v", first)
	}
	wantNext := commitURI(testSHA, "page=2&limit=30")
	if first.FilesNext != wantNext {
		t.Fatalf("files_next = %q, want %q", first.FilesNext, wantNext)
	}

	second, _ := readCommitPage(t, h, first.FilesNext)
	if second.FilesTruncated || second.FilesNext != "" || second.FilesPage != 2 {
		t.Fatalf("page 2 markers: %+v", second)
	}
	if got := append(append([]string(nil), first.Filenames...), second.Filenames...); strings.Join(got, "\n") != strings.Join(paths, "\n") {
		t.Fatalf("pages do not rejoin to the file list: %d + %d files", len(first.Filenames), len(second.Filenames))
	}
}

func TestCommitResourceLimitBelowTheCapPagesSmaller(t *testing.T) {
	t.Parallel()
	h, f := newFilesFixture(t)
	var paths []string
	for i := 0; i < 5; i++ {
		paths = append(paths, fmt.Sprintf("f%d.go", i))
	}
	if err := f.AddCommit(filesOwner, filesRepo, testSHA, "five", paths...); err != nil {
		t.Fatal(err)
	}
	got, _ := readCommitPage(t, h, commitURI(testSHA, "page=2&limit=2"))
	if strings.Join(got.Filenames, ",") != "f2.go,f3.go" || !got.FilesTruncated || got.FilesNext != commitURI(testSHA, "page=3&limit=2") {
		t.Fatalf("page 2 of 2: %+v", got)
	}
}

func TestCommitResourceMarksACutMessage(t *testing.T) {
	t.Parallel()
	h, f := newFilesFixture(t)
	msg := "subject\n\n" + strings.Repeat("x", commitMessageCap*2)
	if err := f.AddCommit(filesOwner, filesRepo, testSHA, msg, "a.go"); err != nil {
		t.Fatal(err)
	}
	got, md := readCommitPage(t, h, commitURI(testSHA, ""))
	if !got.MessageTruncated {
		t.Fatal("json message_truncated must be true for a cut message")
	}
	if !strings.Contains(md, "message truncated") {
		t.Fatalf("markdown sidecar lacks the truncation marker: %.80q...", md)
	}
}

func TestCommitResourceUncutCommitHasNoMarkers(t *testing.T) {
	t.Parallel()
	h, f := newFilesFixture(t)
	if err := f.AddCommit(filesOwner, filesRepo, testSHA, "small", "a.go"); err != nil {
		t.Fatal(err)
	}
	got, md := readCommitPage(t, h, commitURI(testSHA, ""))
	if got.FilesTotal != 0 || got.FilesTruncated || got.FilesNext != "" || got.MessageTruncated || strings.Contains(md, "truncated") {
		t.Fatalf("a small commit carries markers: %+v, %q", got, md)
	}
}
