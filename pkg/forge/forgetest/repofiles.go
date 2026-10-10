// SPDX-License-Identifier: GPL-3.0-or-later

package forgetest

import (
	"encoding/base64"
	"fmt"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

const sha40 = "0123456789abcdef0123456789abcdef01234567"

func b64(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

// RunRepoFiles runs the repository-content contract (files, tree, commits,
// commit statuses) against the implementation newHarness builds. Cases run one
// after another: ChangeFiles reaches Forgejo over the package-level raw-HTTP
// target, which only one stub at a time can be.
func RunRepoFiles(t *testing.T, newHarness func(t *testing.T) Harness) {
	cases := []struct {
		name string
		run  func(t *testing.T, h Harness)
	}{
		{"unknown repository is not found", func(t *testing.T, h Harness) {
			_, err := h.Port.GetFile(ctx, "nobody", "nothing", "main", "a.txt")
			wantNotFound(t, err)
			_, err = h.Port.ListContents(ctx, "nobody", "nothing", "main", "")
			wantNotFound(t, err)
			_, err = h.Port.GetTree(ctx, "nobody", "nothing", "main", forgejo_sdk.GetTreesOptions{})
			wantNotFound(t, err)
			_, err = h.Port.ListCommits(ctx, "nobody", "nothing", forgejo_sdk.ListCommitOptions{})
			wantNotFound(t, err)
			_, err = h.Port.ListCommitStatuses(ctx, "nobody", "nothing", sha40, forgejo_sdk.ListStatusesOption{})
			wantNotFound(t, err)
		}},
		{"raw file and metadata read back", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			must(t, h.Seed.SetFile(Owner, Repo, "main", "docs/a.txt", "hello\nworld"))
			raw, err := h.Port.GetFile(ctx, Owner, Repo, "main", "docs/a.txt")
			must(t, err)
			if string(raw) != "hello\nworld" {
				t.Fatalf("raw = %q", raw)
			}
			c, err := h.Port.GetContents(ctx, Owner, Repo, "main", "docs/a.txt")
			must(t, err)
			if c.Path != "docs/a.txt" || c.Type != "file" || c.SHA == "" || c.Content == nil || *c.Content != b64("hello\nworld") {
				t.Fatalf("contents = %+v", c)
			}
			_, err = h.Port.GetFile(ctx, Owner, Repo, "main", "missing.txt")
			wantNotFound(t, err)
			_, err = h.Port.GetFile(ctx, Owner, Repo, "no-such-branch", "docs/a.txt")
			wantNotFound(t, err)
		}},
		{"directory listing shows one level", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			must(t, h.Seed.SetFile(Owner, Repo, "main", "README.md", "r"))
			must(t, h.Seed.SetFile(Owner, Repo, "main", "docs/a.txt", "a"))
			must(t, h.Seed.SetFile(Owner, Repo, "main", "docs/deep/b.txt", "b"))
			root, err := h.Port.ListContents(ctx, Owner, Repo, "main", "")
			must(t, err)
			wantEntries(t, root, "README.md:file", "docs:dir")
			docs, err := h.Port.ListContents(ctx, Owner, Repo, "main", "docs")
			must(t, err)
			wantEntries(t, docs, "docs/a.txt:file", "docs/deep:dir")
			_, err = h.Port.ListContents(ctx, Owner, Repo, "main", "nope")
			wantNotFound(t, err)
			if _, err = h.Port.GetContents(ctx, Owner, Repo, "main", "docs"); err == nil {
				t.Fatal("contents of a directory must fail")
			}
		}},
		{"tree is top-level unless recursive, and paginates", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			must(t, h.Seed.SetFile(Owner, Repo, "main", "README.md", "r"))
			must(t, h.Seed.SetFile(Owner, Repo, "main", "docs/a.txt", "a"))
			must(t, h.Seed.SetFile(Owner, Repo, "main", "docs/b.txt", "b"))
			top, err := h.Port.GetTree(ctx, Owner, Repo, "main", forgejo_sdk.GetTreesOptions{})
			must(t, err)
			wantTree(t, top, "README.md", "docs")
			all, err := h.Port.GetTree(ctx, Owner, Repo, "main", forgejo_sdk.GetTreesOptions{Recursive: true})
			must(t, err)
			wantTree(t, all, "README.md", "docs/a.txt", "docs/b.txt")
			p1, err := h.Port.GetTree(ctx, Owner, Repo, "main", forgejo_sdk.GetTreesOptions{Recursive: true, ListOptions: forgejo_sdk.ListOptions{Page: 1, PageSize: 2}})
			must(t, err)
			wantTree(t, p1, "README.md", "docs/a.txt")
			p2, err := h.Port.GetTree(ctx, Owner, Repo, "main", forgejo_sdk.GetTreesOptions{Recursive: true, ListOptions: forgejo_sdk.ListOptions{Page: 2, PageSize: 2}})
			must(t, err)
			wantTree(t, p2, "docs/b.txt")
			_, err = h.Port.GetTree(ctx, Owner, Repo, "no-such-ref", forgejo_sdk.GetTreesOptions{})
			wantNotFound(t, err)
		}},
		{"create, update and delete a file round-trip", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			created, err := h.Port.CreateFile(ctx, Owner, Repo, "new.txt", forgejo_sdk.CreateFileOptions{
				FileOptions: forgejo_sdk.FileOptions{Message: "add", BranchName: "main"}, Content: b64("v1"),
			})
			must(t, err)
			if created.Content == nil || created.Content.SHA == "" || created.Commit == nil || created.Commit.SHA == "" {
				t.Fatalf("created = %+v", created)
			}
			raw, err := h.Port.GetFile(ctx, Owner, Repo, "main", "new.txt")
			must(t, err)
			if string(raw) != "v1" {
				t.Fatalf("raw = %q", raw)
			}
			_, err = h.Port.CreateFile(ctx, Owner, Repo, "new.txt", forgejo_sdk.CreateFileOptions{
				FileOptions: forgejo_sdk.FileOptions{Message: "again", BranchName: "main"}, Content: b64("x"),
			})
			if err == nil {
				t.Fatal("creating an existing file must fail")
			}
			_, err = h.Port.UpdateFile(ctx, Owner, Repo, "new.txt", forgejo_sdk.UpdateFileOptions{
				FileOptions: forgejo_sdk.FileOptions{Message: "stale", BranchName: "main"}, SHA: "deadbeef", Content: b64("x"),
			})
			if err == nil {
				t.Fatal("update with a stale sha must fail")
			}
			updated, err := h.Port.UpdateFile(ctx, Owner, Repo, "new.txt", forgejo_sdk.UpdateFileOptions{
				FileOptions: forgejo_sdk.FileOptions{Message: "edit", BranchName: "main"}, SHA: created.Content.SHA, Content: b64("v2"),
			})
			must(t, err)
			raw, _ = h.Port.GetFile(ctx, Owner, Repo, "main", "new.txt")
			if string(raw) != "v2" || updated.Content.SHA == created.Content.SHA {
				t.Fatalf("raw = %q, sha unchanged = %v", raw, updated.Content.SHA == created.Content.SHA)
			}
			must(t, h.Port.DeleteFile(ctx, Owner, Repo, "new.txt", forgejo_sdk.DeleteFileOptions{
				FileOptions: forgejo_sdk.FileOptions{Message: "rm", BranchName: "main"}, SHA: updated.Content.SHA,
			}))
			_, err = h.Port.GetFile(ctx, Owner, Repo, "main", "new.txt")
			wantNotFound(t, err)
			wantNotFound(t, h.Port.DeleteFile(ctx, Owner, Repo, "new.txt", forgejo_sdk.DeleteFileOptions{
				FileOptions: forgejo_sdk.FileOptions{Message: "rm", BranchName: "main"}, SHA: updated.Content.SHA,
			}))
		}},
		{"a write can go to a new branch and leaves the source alone", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			_, err := h.Port.CreateFile(ctx, Owner, Repo, "f.txt", forgejo_sdk.CreateFileOptions{
				FileOptions: forgejo_sdk.FileOptions{Message: "add", BranchName: "main", NewBranchName: "topic"}, Content: b64("x"),
			})
			must(t, err)
			_, err = h.Port.GetFile(ctx, Owner, Repo, "topic", "f.txt")
			must(t, err)
			_, err = h.Port.GetFile(ctx, Owner, Repo, "main", "f.txt")
			wantNotFound(t, err)
		}},
		{"change files applies a batch in one commit", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			created, err := h.Port.CreateFile(ctx, Owner, Repo, "old.txt", forgejo_sdk.CreateFileOptions{
				FileOptions: forgejo_sdk.FileOptions{Message: "seed", BranchName: "main"}, Content: b64("old"),
			})
			must(t, err)
			content := b64("fresh")
			out, err := h.Port.ChangeFiles(ctx, Owner, Repo, forge.ChangeFilesOptions{
				BranchName: "main", Message: "batch",
				Files: []forge.ChangeFileOperation{
					{Operation: "create", Path: "a/new.txt", Content: &content},
					{Operation: "delete", Path: "old.txt", SHA: created.Content.SHA},
				},
			})
			must(t, err)
			if out["commit"] == nil {
				t.Fatalf("answer = %v", out)
			}
			raw, err := h.Port.GetFile(ctx, Owner, Repo, "main", "a/new.txt")
			must(t, err)
			if string(raw) != "fresh" {
				t.Fatalf("raw = %q", raw)
			}
			_, err = h.Port.GetFile(ctx, Owner, Repo, "main", "old.txt")
			wantNotFound(t, err)
			commits, err := h.Port.ListCommits(ctx, Owner, Repo, forgejo_sdk.ListCommitOptions{})
			must(t, err)
			if len(commits) != 2 || commits[0].RepoCommit.Message != "batch" {
				t.Fatalf("want the batch as one newest commit, got %d commits", len(commits))
			}
		}},
		{"a failing change files batch changes nothing", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			content := b64("x")
			_, err := h.Port.ChangeFiles(ctx, Owner, Repo, forge.ChangeFilesOptions{
				BranchName: "main", Message: "bad",
				Files: []forge.ChangeFileOperation{
					{Operation: "create", Path: "ok.txt", Content: &content},
					{Operation: "delete", Path: "ghost.txt", SHA: "abc"},
				},
			})
			if err == nil {
				t.Fatal("want an error")
			}
			_, err = h.Port.GetFile(ctx, Owner, Repo, "main", "ok.txt")
			wantNotFound(t, err)
		}},
		{"change files in an unknown repository is not found", func(t *testing.T, h Harness) {
			content := b64("x")
			_, err := h.Port.ChangeFiles(ctx, "nobody", "nothing", forge.ChangeFilesOptions{
				BranchName: "main", Message: "m", Files: []forge.ChangeFileOperation{{Operation: "create", Path: "a", Content: &content}},
			})
			wantNotFound(t, err)
		}},
		{"commits list newest first, filter by path and paginate", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			must(t, h.Seed.AddCommit(Owner, Repo, sha40, "first", "a.txt"))
			must(t, h.Seed.AddCommit(Owner, Repo, "1123456789abcdef0123456789abcdef01234567", "second", "b.txt"))
			must(t, h.Seed.AddCommit(Owner, Repo, "2123456789abcdef0123456789abcdef01234567", "third", "a.txt"))
			all, err := h.Port.ListCommits(ctx, Owner, Repo, forgejo_sdk.ListCommitOptions{})
			must(t, err)
			wantMessages(t, all, "third", "second", "first")
			onlyA, err := h.Port.ListCommits(ctx, Owner, Repo, forgejo_sdk.ListCommitOptions{Path: "a.txt"})
			must(t, err)
			wantMessages(t, onlyA, "third", "first")
			p2, err := h.Port.ListCommits(ctx, Owner, Repo, forgejo_sdk.ListCommitOptions{ListOptions: forgejo_sdk.ListOptions{Page: 2, PageSize: 2}})
			must(t, err)
			wantMessages(t, p2, "first")
		}},
		{"a commit reads back by sha", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			must(t, h.Seed.AddCommit(Owner, Repo, sha40, "the message", "a.txt"))
			c, err := h.Port.GetCommit(ctx, Owner, Repo, sha40)
			must(t, err)
			if c.CommitMeta == nil || c.SHA != sha40 || c.RepoCommit == nil || c.RepoCommit.Message != "the message" {
				t.Fatalf("commit = %+v", c)
			}
			_, err = h.Port.GetCommit(ctx, Owner, Repo, "ffffffffffffffffffffffffffffffffffffffff")
			wantNotFound(t, err)
		}},
		{"commit statuses paginate and report the total", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			must(t, h.Seed.AddCommit(Owner, Repo, sha40, "c"))
			empty, err := h.Port.ListCommitStatuses(ctx, Owner, Repo, sha40, forgejo_sdk.ListStatusesOption{})
			must(t, err)
			if len(empty.Statuses) != 0 || empty.TotalCount == nil || *empty.TotalCount != 0 {
				t.Fatalf("empty = %+v", empty)
			}
			for i := 1; i <= 3; i++ {
				must(t, h.Seed.AddCommitStatus(Owner, Repo, sha40, fmt.Sprintf("ci/%d", i), forgejo_sdk.StatusSuccess, "ok"))
			}
			p1, err := h.Port.ListCommitStatuses(ctx, Owner, Repo, sha40, forgejo_sdk.ListStatusesOption{ListOptions: forgejo_sdk.ListOptions{Page: 1, PageSize: 2}})
			must(t, err)
			if len(p1.Statuses) != 2 || p1.TotalCount == nil || *p1.TotalCount != 3 {
				t.Fatalf("page 1 = %d statuses, total %v", len(p1.Statuses), p1.TotalCount)
			}
			if p1.Statuses[0].Context != "ci/1" || p1.Statuses[0].State != forgejo_sdk.StatusSuccess {
				t.Fatalf("first = %+v", p1.Statuses[0])
			}
			p2, err := h.Port.ListCommitStatuses(ctx, Owner, Repo, sha40, forgejo_sdk.ListStatusesOption{ListOptions: forgejo_sdk.ListOptions{Page: 2, PageSize: 2}})
			must(t, err)
			if len(p2.Statuses) != 1 || p2.Statuses[0].Context != "ci/3" {
				t.Fatalf("page 2 = %+v", p2.Statuses)
			}
			_, err = h.Port.ListCommitStatuses(ctx, Owner, Repo, "ffffffffffffffffffffffffffffffffffffffff", forgejo_sdk.ListStatusesOption{})
			wantNotFound(t, err)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.run(t, newHarness(t))
		})
	}
}

func wantEntries(t *testing.T, got []*forgejo_sdk.ContentsResponse, want ...string) {
	t.Helper()
	have := make([]string, len(got))
	for i, c := range got {
		have[i] = c.Path + ":" + c.Type
	}
	if fmt.Sprint(have) != fmt.Sprint(want) {
		t.Fatalf("got entries %v, want %v", have, want)
	}
}

func wantTree(t *testing.T, got *forgejo_sdk.GitTreeResponse, want ...string) {
	t.Helper()
	have := make([]string, len(got.Entries))
	for i, e := range got.Entries {
		have[i] = e.Path
	}
	if fmt.Sprint(have) != fmt.Sprint(want) {
		t.Fatalf("got tree %v, want %v", have, want)
	}
}

func wantMessages(t *testing.T, got []*forgejo_sdk.Commit, want ...string) {
	t.Helper()
	have := make([]string, len(got))
	for i, c := range got {
		have[i] = c.RepoCommit.Message
	}
	if fmt.Sprint(have) != fmt.Sprint(want) {
		t.Fatalf("got commits %v, want %v", have, want)
	}
}
