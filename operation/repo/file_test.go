// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func newCallToolRequest(args map[string]interface{}) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}
}

// fileOnMain reads a file back through the port, as the tool's caller would.
func fileOnMain(t *testing.T, h handlers, filePath string) []byte {
	t.Helper()
	port, err := h.forge.For(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	b, err := port.GetFile(context.Background(), filesOwner, filesRepo, "main", filePath)
	if err != nil {
		t.Fatalf("reading %s back: %v", filePath, err)
	}
	return b
}

func TestCreateFileFn_StoresPlainContent(t *testing.T) {
	t.Parallel()
	h, _ := newFilesFixture(t)
	plainText := "Hello, World!\nThis is a test file."

	result, err := h.CreateFileFn(context.Background(), filesReq(map[string]any{
		"filePath": "test.txt", "content": plainText, "message": "add test file", "branch_name": "main",
	}))
	if err != nil || result.IsError {
		t.Fatalf("err=%v result=%v", err, result)
	}
	if got := string(fileOnMain(t, h, "test.txt")); got != plainText {
		t.Errorf("stored %q, want %q", got, plainText)
	}
}

func TestUpdateFileFn_StoresPlainContent(t *testing.T) {
	t.Parallel()
	h, f := newFilesFixture(t)
	seedFile(t, f, "test.txt", "old")
	sha := currentSHA(t, h, "test.txt")
	plainText := "Updated content with special chars: <>&\"\n\ttabs too"

	result, err := h.UpdateFileFn(context.Background(), filesReq(map[string]any{
		"filePath": "test.txt", "content": plainText, "message": "update", "branch_name": "main", "sha": sha,
	}))
	if err != nil || result.IsError {
		t.Fatalf("err=%v result=%v", err, result)
	}
	if got := string(fileOnMain(t, h, "test.txt")); got != plainText {
		t.Errorf("stored %q, want %q", got, plainText)
	}
}

func currentSHA(t *testing.T, h handlers, filePath string) string {
	t.Helper()
	port, _ := h.forge.For(context.Background())
	c, err := port.GetContents(context.Background(), filesOwner, filesRepo, "main", filePath)
	if err != nil {
		t.Fatal(err)
	}
	return c.SHA
}

func TestUpdateFileFn_StaleSHAIsAToolError(t *testing.T) {
	t.Parallel()
	h, f := newFilesFixture(t)
	seedFile(t, f, "test.txt", "old")

	_, err := h.UpdateFileFn(context.Background(), filesReq(map[string]any{
		"filePath": "test.txt", "content": "new", "message": "m", "branch_name": "main", "sha": "stale",
	}))
	if err == nil || !strings.Contains(err.Error(), "update file error") {
		t.Fatalf("want an update file error, got %v", err)
	}
	if got := string(fileOnMain(t, h, "test.txt")); got != "old" {
		t.Errorf("file changed to %q", got)
	}
}

func TestDeleteFileFn_RemovesFile(t *testing.T) {
	t.Parallel()
	h, f := newFilesFixture(t)
	seedFile(t, f, "gone.txt", "bye")

	result, err := h.DeleteFileFn(context.Background(), filesReq(map[string]any{
		"filePath": "gone.txt", "message": "rm", "branch_name": "main", "sha": currentSHA(t, h, "gone.txt"),
	}))
	if err != nil || result.IsError {
		t.Fatalf("err=%v result=%v", err, result)
	}
	if got := resultText(t, result); got != "Delete file success" {
		t.Errorf("result %q", got)
	}
	port, _ := h.forge.For(context.Background())
	if _, err := port.GetFile(context.Background(), filesOwner, filesRepo, "main", "gone.txt"); err == nil {
		t.Error("file still there")
	}
}

func TestGetFileContentFn_ReturnsPlainText(t *testing.T) {
	t.Parallel()
	h, f := newFilesFixture(t)
	plainText := "Hello from Forgejo!\nSecond line.\n"
	seedFile(t, f, "README.md", plainText)

	result, err := h.GetFileContentFn(context.Background(), filesReq(map[string]any{"ref": "main", "filePath": "README.md"}))
	if err != nil || result.IsError {
		t.Fatalf("err=%v result=%v", err, result)
	}
	if got := resultText(t, result); got != plainText {
		t.Errorf("got %q want %q", got, plainText)
	}
}

func TestGetFileContentFn_MissingFileIsAToolError(t *testing.T) {
	t.Parallel()
	h, _ := newFilesFixture(t)

	_, err := h.GetFileContentFn(context.Background(), filesReq(map[string]any{"ref": "main", "filePath": "nope.md"}))
	if err == nil || !strings.Contains(err.Error(), "get file err") {
		t.Fatalf("want a get file err, got %v", err)
	}
}

func TestGetFileContentFn_WithMetadataReturnsContentsResponse(t *testing.T) {
	t.Parallel()
	h, f := newFilesFixture(t)
	seedFile(t, f, "README.md", "binary-like content")

	result, err := h.GetFileContentFn(context.Background(), filesReq(map[string]any{
		"ref": "main", "filePath": "README.md", "with_metadata": true,
	}))
	if err != nil || result.IsError {
		t.Fatalf("err=%v result=%v", err, result)
	}
	text := extractText(t, result)
	for _, want := range []string{`"sha":"` + currentSHA(t, h, "README.md") + `"`, `"path":"README.md"`, base64.StdEncoding.EncodeToString([]byte("binary-like content"))} {
		if !strings.Contains(text, want) {
			t.Errorf("response missing %s: %q", want, text)
		}
	}
}

func rangeResult(t *testing.T, body string, args map[string]any) (string, error) {
	t.Helper()
	h, f := newFilesFixture(t)
	seedFile(t, f, "f.txt", body)
	args["ref"], args["filePath"] = "main", "f.txt"
	result, err := h.GetFileContentFn(context.Background(), filesReq(args))
	if err != nil {
		return "", err
	}
	return resultText(t, result), nil
}

func TestGetFileContentFn_LineRange(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
		args map[string]any
		want string
	}{
		{"in range", "one\ntwo\nthree\nfour\nfive\n", map[string]any{"start_line": float64(2), "end_line": float64(4)}, "two\nthree\nfour"},
		{"start omitted defaults to one", "one\ntwo\nthree\nfour", map[string]any{"end_line": float64(2)}, "one\ntwo"},
		{"end omitted defaults to count", "one\ntwo\nthree\nfour", map[string]any{"start_line": float64(3)}, "three\nfour"},
		{"end beyond clamps", "one\ntwo\nthree", map[string]any{"start_line": float64(2), "end_line": float64(999)}, "two\nthree"},
		{"start below one clamps", "one\ntwo\nthree", map[string]any{"start_line": float64(-5), "end_line": float64(2)}, "one\ntwo"},
		{"both omitted returns full", "one\ntwo\nthree\n", map[string]any{}, "one\ntwo\nthree\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := rangeResult(t, tt.body, tt.args)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestGetFileContentFn_LineRange_InvertedAfterClamping(t *testing.T) {
	t.Parallel()
	if _, err := rangeResult(t, "one\ntwo\nthree", map[string]any{"start_line": float64(20), "end_line": float64(10)}); err == nil {
		t.Fatal("expected error on inverted range")
	}
}

func TestGetFileContentFn_LineRange_IgnoredWithMetadata(t *testing.T) {
	t.Parallel()
	h, f := newFilesFixture(t)
	seedFile(t, f, "README.md", "one\ntwo")

	result, err := h.GetFileContentFn(context.Background(), filesReq(map[string]any{
		"ref": "main", "filePath": "README.md", "with_metadata": true,
		"start_line": float64(5), "end_line": float64(10),
	}))
	if err != nil || result.IsError {
		t.Fatalf("err=%v result=%v", err, result)
	}
	if !strings.Contains(extractText(t, result), `"sha":"`) {
		t.Fatalf("expected ContentsResponse with sha when with_metadata=true, got %q", extractText(t, result))
	}
}

func TestCreateFileFn_AcceptsBase64Content(t *testing.T) {
	t.Parallel()
	h, _ := newFilesFixture(t)
	raw := []byte{0x00, 0xff, 0x10, 0x80}

	result, err := h.CreateFileFn(context.Background(), filesReq(map[string]any{
		"filePath": "binary.dat", "content_base64": base64.StdEncoding.EncodeToString(raw),
		"message": "add binary file", "branch_name": "main",
	}))
	if err != nil || result.IsError {
		t.Fatalf("err=%v result=%v", err, result)
	}
	if got := fileOnMain(t, h, "binary.dat"); string(got) != string(raw) {
		t.Errorf("stored % x, want % x", got, raw)
	}
}

func TestUpdateFileFn_AcceptsBase64Content(t *testing.T) {
	t.Parallel()
	h, f := newFilesFixture(t)
	seedFile(t, f, "binary.dat", "old")
	raw := []byte{0xde, 0xad, 0xbe, 0xef}

	result, err := h.UpdateFileFn(context.Background(), filesReq(map[string]any{
		"filePath": "binary.dat", "content_base64": base64.StdEncoding.EncodeToString(raw),
		"message": "update binary file", "branch_name": "main", "sha": currentSHA(t, h, "binary.dat"),
	}))
	if err != nil || result.IsError {
		t.Fatalf("err=%v result=%v", err, result)
	}
	if got := fileOnMain(t, h, "binary.dat"); string(got) != string(raw) {
		t.Errorf("stored % x, want % x", got, raw)
	}
}

func TestRepositoryWriteContentRejectsMissingAndAmbiguousContent(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args map[string]any
	}{
		{name: "missing", args: map[string]any{}},
		{name: "both", args: map[string]any{"content": "text", "content_base64": "dGV4dA=="}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := selectFromMap(tt.args); err == nil {
				t.Fatal("expected content selection error")
			}
		})
	}
}

func TestRepositoryWriteContentRejectsInvalidBase64(t *testing.T) {
	t.Parallel()
	if _, err := selectFromMap(map[string]any{"content_base64": "%%%not-base64%%%"}); err == nil {
		t.Fatal("expected invalid base64 error")
	}
}

func TestRepositoryWriteContentAcceptsExplicitEmptyValues(t *testing.T) {
	t.Parallel()
	for _, args := range []map[string]any{
		{"content": ""},
		{"content_base64": ""},
	} {
		got, err := selectFromMap(args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "" {
			t.Fatalf("expected zero-byte encoded content, got %q", got)
		}
	}
}
