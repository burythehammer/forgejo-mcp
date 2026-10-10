package repo

import (
	"context"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestListRepoContentsFn_ReturnsContents(t *testing.T) {
	t.Parallel()
	h, f := newFilesFixture(t)
	seedFile(t, f, "README.md", "hi")
	seedFile(t, f, "src/main.go", "package main")

	result, err := h.ListRepoContentsFn(context.Background(), filesReq(map[string]any{"ref": "main", "path": ""}))
	if err != nil || result.IsError {
		t.Fatalf("err=%v result=%v", err, result)
	}
	text := extractText(t, result)
	if !strings.Contains(text, "README.md") || !strings.Contains(text, `"type":"dir"`) || !strings.Contains(text, "src") {
		t.Errorf("result should list the file and the directory\n  got: %s", text)
	}
	if strings.Contains(text, "main.go") {
		t.Errorf("only one level should be listed\n  got: %s", text)
	}
}

func TestListRepoContentsFn_UnknownPathIsAToolError(t *testing.T) {
	t.Parallel()
	h, _ := newFilesFixture(t)

	_, err := h.ListRepoContentsFn(context.Background(), filesReq(map[string]any{"ref": "main", "path": "nope"}))
	if err == nil || !strings.Contains(err.Error(), "list repo contents err") {
		t.Fatalf("want a list repo contents err, got %v", err)
	}
}

func TestGetRepoTreeFn_ReturnsTree(t *testing.T) {
	t.Parallel()
	h, f := newFilesFixture(t)
	seedFile(t, f, "README.md", "hi")
	seedFile(t, f, "src/main.go", "package main")

	result, err := h.GetRepoTreeFn(context.Background(), filesReq(map[string]any{"ref": "main", "recursive": true}))
	if err != nil || result.IsError {
		t.Fatalf("err=%v result=%v", err, result)
	}
	text := extractText(t, result)
	if !strings.Contains(text, "src/main.go") || !strings.Contains(text, "README.md") {
		t.Errorf("recursive tree should list every path\n  got: %s", text)
	}
}

func TestGetRepoTreeFn_NotRecursiveListsTopLevelOnly(t *testing.T) {
	t.Parallel()
	h, f := newFilesFixture(t)
	seedFile(t, f, "README.md", "hi")
	seedFile(t, f, "src/main.go", "package main")

	result, err := h.GetRepoTreeFn(context.Background(), filesReq(map[string]any{"ref": "main"}))
	if err != nil || result.IsError {
		t.Fatalf("err=%v result=%v", err, result)
	}
	if text := extractText(t, result); strings.Contains(text, "main.go") {
		t.Errorf("non-recursive tree should not descend\n  got: %s", text)
	}
}

// extractText pulls the string content from a CallToolResult.
func extractText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	for _, c := range result.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			return tc.Text
		}
	}
	t.Fatalf("no text content in result")
	return ""
}
