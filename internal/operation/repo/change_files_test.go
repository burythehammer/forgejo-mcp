// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"
	"strings"
	"testing"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/decode"
)

func TestChangeFilesFnAppliesAtomicBatch(t *testing.T) {
	t.Parallel()
	h, f := newFilesFixture(t)
	seedFile(t, f, "binary.dat", "old bytes")
	seedFile(t, f, "old.txt", "doomed")

	result, err := h.ChangeFilesFn(context.Background(), filesReq(map[string]any{
		"message":     "atomic change",
		"branch_name": "main",
		"files": []any{
			map[string]any{"operation": "create", "path": "new.txt", "content": "hello"},
			map[string]any{"operation": "update", "path": "binary.dat", "content_base64": "AP8=", "sha": currentSHA(t, h, "binary.dat")},
			map[string]any{"operation": "delete", "path": "old.txt", "sha": currentSHA(t, h, "old.txt")},
		},
	}))
	if err != nil || result.IsError {
		t.Fatalf("err=%v result=%v", err, result)
	}

	if got := string(fileOnMain(t, h, "new.txt")); got != "hello" {
		t.Errorf("new.txt = %q, want plain text to be stored", got)
	}
	if got := fileOnMain(t, h, "binary.dat"); string(got) != "\x00\xff" {
		t.Errorf("binary.dat = % x, want 00 ff", got)
	}
	port, _ := h.forge.For(context.Background())
	if _, err := port.GetFile(context.Background(), filesOwner, filesRepo, "main", "old.txt"); err == nil {
		t.Error("old.txt should be deleted")
	}
	commits, err := port.ListCommits(context.Background(), filesOwner, filesRepo, forgejo_sdk.ListCommitOptions{})
	if err != nil || len(commits) != 1 || commits[0].RepoCommit.Message != "atomic change" {
		t.Errorf("want the batch as exactly one commit, got %v (err %v)", commits, err)
	}
}

func TestChangeFilesFnFailureChangesNothing(t *testing.T) {
	t.Parallel()
	h, _ := newFilesFixture(t)

	_, err := h.ChangeFilesFn(context.Background(), filesReq(map[string]any{
		"message": "m", "branch_name": "main",
		"files": []any{
			map[string]any{"operation": "create", "path": "ok.txt", "content": "x"},
			map[string]any{"operation": "delete", "path": "ghost.txt", "sha": "abc"},
		},
	}))
	if err == nil || !strings.Contains(err.Error(), "change files error") {
		t.Fatalf("want a change files error, got %v", err)
	}
	port, _ := h.forge.For(context.Background())
	if _, err := port.GetFile(context.Background(), filesOwner, filesRepo, "main", "ok.txt"); err == nil {
		t.Error("a failed batch must not write ok.txt")
	}
}

func TestParseChangeFilesArgumentsRejectsInvalidBatches(t *testing.T) {
	t.Parallel()
	base := func(files []any) map[string]any {
		return map[string]any{
			"owner":       "o",
			"repo":        "r",
			"message":     "m",
			"branch_name": "main",
			"files":       files,
		}
	}

	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{
			name: "empty batch",
			args: base([]any{}),
			want: "at least one",
		},
		{
			name: "duplicate path",
			args: base([]any{
				map[string]any{"operation": "create", "path": "a", "content": "x"},
				map[string]any{"operation": "create", "path": "a", "content": "y"},
			}),
			want: "duplicate target path",
		},
		{
			name: "create with sha",
			args: base([]any{
				map[string]any{
					"operation": "create",
					"path":      "a",
					"content":   "x",
					"sha":       "abc",
				},
			}),
			want: "sha must be omitted",
		},
		{
			name: "update without sha",
			args: base([]any{
				map[string]any{
					"operation": "update",
					"path":      "a",
					"content":   "x",
				},
			}),
			want: "sha is required",
		},
		{
			name: "delete without sha",
			args: base([]any{
				map[string]any{"operation": "delete", "path": "a"},
			}),
			want: "sha is required",
		},
		{
			name: "delete with content",
			args: base([]any{
				map[string]any{
					"operation": "delete",
					"path":      "a",
					"sha":       "abc",
					"content":   "",
				},
			}),
			want: "content must be omitted",
		},
		{
			name: "invalid operation",
			args: base([]any{
				map[string]any{"operation": "rename", "path": "a"},
			}),
			want: "unsupported operation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var a changeFilesArgs
			if err := decode.Map(tt.args, &a); err != nil {
				t.Fatal(err)
			}
			_, _, _, err := parseChangeFiles(a)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error %q does not contain %q", err, tt.want)
			}
		})
	}
}
