// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"bytes"
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/model"
)

// contentsGit is a fakeGit that also answers GetContents with a fixed entry.
type contentsGit struct {
	*fakeGit
	entry *model.ContentsResponse
}

func (g contentsGit) GetContents(context.Context, string, string, string, string) (*model.ContentsResponse, error) {
	return g.entry, nil
}

func entryFor(body []byte) *model.ContentsResponse {
	enc, content := "base64", base64.StdEncoding.EncodeToString(body)
	return &model.ContentsResponse{Type: "file", Size: int64(len(body)), Encoding: &enc, Content: &content}
}

func TestGetFileTextRefusesAWholeFileOverTheInlineLimit(t *testing.T) {
	t.Parallel()
	svc := NewGitService(&fakeGit{file: bytes.Repeat([]byte("x\n"), InlineFileBytes/2+1)})
	_, err := svc.GetFileText(context.Background(), "o", "r", "main", "p", LineRange{})
	if err == nil || !strings.Contains(err.Error(), "start_line") || !strings.Contains(err.Error(), "end_line") {
		t.Fatalf("want a refusal that names start_line and end_line, got %v", err)
	}
}

func TestGetFileTextRefusesARangeWhoseTextIsOverTheInlineLimit(t *testing.T) {
	t.Parallel()
	svc := NewGitService(&fakeGit{file: []byte(strings.Repeat("x", InlineFileBytes+10))})
	_, err := svc.GetFileText(context.Background(), "o", "r", "main", "p", LineRange{Start: 1, End: 1})
	if err == nil || !strings.Contains(err.Error(), "narrow") {
		t.Fatalf("want a refusal asking for a narrower range, got %v", err)
	}
}

func TestGetFileTextReturnsASmallRangeOfALargeFile(t *testing.T) {
	t.Parallel()
	body := append([]byte("first\nsecond\n"), bytes.Repeat([]byte("y"), InlineFileBytes)...)
	svc := NewGitService(&fakeGit{file: body})
	got, err := svc.GetFileText(context.Background(), "o", "r", "main", "p", LineRange{Start: 1, End: 2})
	if err != nil || got != "first\nsecond" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestGetFileMetadataRefusesAWholeFileOverTheInlineLimit(t *testing.T) {
	t.Parallel()
	svc := NewGitService(contentsGit{fakeGit: &fakeGit{}, entry: entryFor(bytes.Repeat([]byte("x"), InlineFileBytes+1))})
	_, err := svc.GetFileMetadata(context.Background(), "o", "r", "main", "p", LineRange{})
	if err == nil || !strings.Contains(err.Error(), "start_line") {
		t.Fatalf("want a refusal that names start_line, got %v", err)
	}
}

func TestGetFileMetadataSlicesContentToTheRangeAndKeepsTheMetadata(t *testing.T) {
	t.Parallel()
	entry := entryFor([]byte("a\nb\nc\n"))
	svc := NewGitService(contentsGit{fakeGit: &fakeGit{}, entry: entry})
	got, err := svc.GetFileMetadata(context.Background(), "o", "r", "main", "p", LineRange{Start: 2, End: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	dec, derr := base64.StdEncoding.DecodeString(*got.Content)
	if derr != nil || string(dec) != "b\nc" {
		t.Fatalf("content: got %q, %v; want %q", dec, derr, "b\nc")
	}
	if got.Size != 6 || got.Encoding == nil || *got.Encoding != "base64" {
		t.Fatalf("metadata changed: size %d, encoding %v", got.Size, got.Encoding)
	}
}

func TestGetFileMetadataSmallFileWithoutRangeIsUnchanged(t *testing.T) {
	t.Parallel()
	entry := entryFor([]byte("a\nb\n"))
	svc := NewGitService(contentsGit{fakeGit: &fakeGit{}, entry: entry})
	got, err := svc.GetFileMetadata(context.Background(), "o", "r", "main", "p", LineRange{})
	if err != nil || got != entry || *got.Content != *entry.Content {
		t.Fatalf("got %+v, %v", got, err)
	}
}
