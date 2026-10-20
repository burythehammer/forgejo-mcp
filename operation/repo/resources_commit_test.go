// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/operation/resource"

	"github.com/mark3labs/mcp-go/mcp"
)

const testSHA = "a3f1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9"

func makeCommitResourceRequest(sha string) mcp.ReadResourceRequest {
	return mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{
			URI: "forgejo://repo/" + filesOwner + "/" + filesRepo + "/commit/" + sha,
		},
	}
}

func TestCommitResourceHandler_HappyPath(t *testing.T) {
	t.Parallel()
	h, f := newFilesFixture(t)
	if err := f.AddCommit(filesOwner, filesRepo, testSHA, "fix: resolve race condition\n\nDetailed explanation.", "a.go"); err != nil {
		t.Fatal(err)
	}

	contents, err := h.commitResourceHandler(context.Background(), makeCommitResourceRequest(testSHA))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(contents) < 2 {
		t.Fatalf("expected at least 2 content blocks (JSON + markdown), got %d", len(contents))
	}

	jsonBlock, ok := contents[0].(mcp.TextResourceContents)
	if !ok {
		t.Fatal("first block must be TextResourceContents")
	}
	if jsonBlock.MIMEType != "application/json" {
		t.Errorf("first block MIME type: got %q, want application/json", jsonBlock.MIMEType)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(jsonBlock.Text), &decoded); err != nil {
		t.Errorf("first block is not valid JSON: %v", err)
	}
	if decoded["sha"] != testSHA {
		t.Errorf("json sha = %v, want %s", decoded["sha"], testSHA)
	}

	mdBlock, ok := contents[1].(mcp.TextResourceContents)
	if !ok {
		t.Fatal("second block must be TextResourceContents")
	}
	if mdBlock.MIMEType != "text/markdown" {
		t.Errorf("second block MIME type: got %q, want text/markdown", mdBlock.MIMEType)
	}
	if !strings.Contains(mdBlock.Text, "race condition") {
		t.Errorf("markdown sidecar missing commit message, got %q", mdBlock.Text)
	}
}

func TestCommitResourceHandler_ShortSHA(t *testing.T) {
	t.Parallel()
	h, _ := newFilesFixture(t)
	_, err := h.commitResourceHandler(context.Background(), makeCommitResourceRequest("abc123"))
	if err == nil {
		t.Fatal("expected error for short sha")
	}
	// Parse errors surface as *resource.ResourceError with Code -32602 (invalid params)
	// because the ErrInvalidParams sentinel is wrapped at every parser return path.
	var resErr *resource.ResourceError
	if !errors.As(err, &resErr) {
		t.Fatalf("expected *resource.ResourceError, got %T: %v", err, err)
	}
	if resErr.Code != -32602 {
		t.Errorf("expected Code -32602, got %d", resErr.Code)
	}
}

func TestCommitResourceHandler_NotFound(t *testing.T) {
	t.Parallel()
	h, _ := newFilesFixture(t)
	_, err := h.commitResourceHandler(context.Background(), makeCommitResourceRequest(testSHA))
	var resErr *resource.ResourceError
	if !errors.As(err, &resErr) || resErr.Code != -32003 {
		t.Fatalf("want a not-found resource error (-32003), got %T %v", err, err)
	}
}

func TestCommitResourceHandler_SidecarPresent(t *testing.T) {
	t.Parallel()
	h, f := newFilesFixture(t)
	if err := f.AddCommit(filesOwner, filesRepo, testSHA, "chore: update deps"); err != nil {
		t.Fatal(err)
	}

	contents, err := h.commitResourceHandler(context.Background(), makeCommitResourceRequest(testSHA))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, c := range contents {
		if tc, ok := c.(mcp.TextResourceContents); ok && tc.MIMEType == "text/markdown" {
			found = true
		}
	}
	if !found {
		t.Error("expected text/markdown sidecar block to be present")
	}
}

func TestCommitResourceHandler_BoundsFilesAndMessage(t *testing.T) {
	t.Parallel()
	h, f := newFilesFixture(t)
	paths := make([]string, 45)
	for i := range paths {
		paths[i] = fmt.Sprintf("dir/file%02d.go", i)
	}
	msg := "subject\n\n" + strings.Repeat("x", commitMessageCap*2)
	if err := f.AddCommit(filesOwner, filesRepo, testSHA, msg, paths...); err != nil {
		t.Fatal(err)
	}

	contents, err := h.commitResourceHandler(context.Background(), makeCommitResourceRequest(testSHA))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var decoded struct {
		Files          []map[string]any `json:"files"`
		FilesTotal     int              `json:"files_total"`
		FilesTruncated bool             `json:"files_truncated"`
		Commit         struct {
			Message string `json:"message"`
		} `json:"commit"`
	}
	if err := json.Unmarshal([]byte(contents[0].(mcp.TextResourceContents).Text), &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Files) != resource.EmbeddedListCap || decoded.FilesTotal != 45 || !decoded.FilesTruncated {
		t.Errorf("files = %d total=%d truncated=%v, want %d/45/true", len(decoded.Files), decoded.FilesTotal, decoded.FilesTruncated, resource.EmbeddedListCap)
	}
	if len(decoded.Commit.Message) > commitMessageCap+8 {
		t.Errorf("json message is %d bytes, want at most about %d", len(decoded.Commit.Message), commitMessageCap)
	}
	if md := contents[1].(mcp.TextResourceContents).Text; len(md) > commitMessageCap+8 {
		t.Errorf("markdown sidecar is %d bytes, want at most about %d", len(md), commitMessageCap)
	}
}

func TestCommitResourceHandler_SmallCommitUnchanged(t *testing.T) {
	t.Parallel()
	h, f := newFilesFixture(t)
	if err := f.AddCommit(filesOwner, filesRepo, testSHA, "small", "a.go"); err != nil {
		t.Fatal(err)
	}
	contents, err := h.commitResourceHandler(context.Background(), makeCommitResourceRequest(testSHA))
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(contents[0].(mcp.TextResourceContents).Text), &decoded); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"files_total", "files_truncated"} {
		if _, ok := decoded[k]; ok {
			t.Errorf("%s must be absent when nothing is truncated", k)
		}
	}
}
