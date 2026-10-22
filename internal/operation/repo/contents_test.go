// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"
	"encoding/json"
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

// TestGetRepoTreeFn_EnvelopeHoldsEntriesUnderTree pins the bounded envelope: the
// entries sit under tree, and has_next comes from Forgejo's truncation report.
func TestGetRepoTreeFn_EnvelopeHoldsEntriesUnderTree(t *testing.T) {
	t.Parallel()
	h, f := newFilesFixture(t)
	seedFile(t, f, "a.md", "a")
	seedFile(t, f, "b.md", "b")
	seedFile(t, f, "c.md", "c")

	result, err := h.GetRepoTreeFn(context.Background(), filesReq(map[string]any{"ref": "main", "limit": float64(2)}))
	if err != nil || result.IsError {
		t.Fatalf("err=%v result=%v", err, result)
	}
	var wrapped struct {
		Result struct {
			Tree    []map[string]any `json:"tree"`
			Page    int              `json:"page"`
			Limit   int              `json:"limit"`
			Count   int              `json:"count"`
			HasNext bool             `json:"has_next"`
			Total   *int             `json:"total_count"`
		} `json:"Result"`
	}
	if err := json.Unmarshal([]byte(extractText(t, result)), &wrapped); err != nil {
		t.Fatalf("not the envelope: %v", err)
	}
	env := wrapped.Result
	if len(env.Tree) != 2 || env.Count != 2 || env.Page != 1 || env.Limit != 2 {
		t.Errorf("window = tree %d count %d page %d limit %d, want 2 2 1 2", len(env.Tree), env.Count, env.Page, env.Limit)
	}
	if !env.HasNext {
		t.Errorf("has_next = false, want true: Forgejo truncated the page")
	}
	if env.Total == nil || *env.Total != 3 {
		t.Errorf("total_count = %v, want 3", env.Total)
	}
}

// TestGetRepoTreeFn_LastPageHasNoNext: the final page reports no rows beyond it.
func TestGetRepoTreeFn_LastPageHasNoNext(t *testing.T) {
	t.Parallel()
	h, f := newFilesFixture(t)
	seedFile(t, f, "a.md", "a")
	seedFile(t, f, "b.md", "b")

	result, err := h.GetRepoTreeFn(context.Background(), filesReq(map[string]any{"ref": "main", "limit": float64(2)}))
	if err != nil || result.IsError {
		t.Fatalf("err=%v result=%v", err, result)
	}
	if text := extractText(t, result); !strings.Contains(text, `"has_next":false`) {
		t.Errorf("full last page should say has_next false\n  got: %s", text)
	}
}

// TestGetRepoTreeFn_OverCeilingRefused: a limit above a known ceiling is refused
// and the ceiling is named, as every other list does.
func TestGetRepoTreeFn_OverCeilingRefused(t *testing.T) {
	t.Parallel()
	h, f := newFilesFixture(t)
	seedFile(t, f, "a.md", "a")
	f.SetMaxResponseItems(2)

	result, err := h.GetRepoTreeFn(context.Background(), filesReq(map[string]any{"ref": "main", "limit": float64(5)}))
	if err == nil && (result == nil || !result.IsError) {
		t.Fatalf("limit 5 over a ceiling of 2 was not refused")
	}
	var msg string
	if err != nil {
		msg = err.Error()
	} else {
		msg = extractText(t, result)
	}
	if !strings.Contains(msg, "max_response_items") {
		t.Errorf("refusal does not name the ceiling: %s", msg)
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
