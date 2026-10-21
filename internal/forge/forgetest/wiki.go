// SPDX-License-Identifier: GPL-3.0-or-later

package forgetest

import (
	"fmt"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
)

// newWikiPage creates a page in Owner/Repo, making the repository first.
func newWikiPage(t *testing.T, h Harness, title, content string) *forge.WikiPage {
	t.Helper()
	h.Seed.AddRepo(Owner, Repo)
	p, err := h.Port.CreateWikiPage(ctx, Owner, Repo, title, b64(content), "create "+title)
	must(t, err)
	return p
}

// RunWiki runs the wiki contract against the implementation that newHarness
// builds. newHarness is called once per case.
func RunWiki(t *testing.T, newHarness func(t *testing.T) Harness) {
	cases := []struct {
		name string
		run  func(t *testing.T, h Harness)
	}{
		{"an unknown repository lists no pages and reads nothing", func(t *testing.T, h Harness) {
			got, err := h.Port.ListWikiPages(ctx, "nobody", "nothing", 1, 30)
			must(t, err)
			if got.Pages == nil || len(got.Pages) != 0 {
				t.Fatalf("want an empty non-nil list, got %#v", got.Pages)
			}
			_, err = h.Port.GetWikiPage(ctx, "nobody", "nothing", "Home")
			wantNotFound(t, err)
			_, err = h.Port.CreateWikiPage(ctx, "nobody", "nothing", "Home", b64("x"), "m")
			wantNotFound(t, err)
			_, err = h.Port.GetWikiPageRevisions(ctx, "nobody", "nothing", "Home", 1, 30)
			wantNotFound(t, err)
		}},
		{"a repository without pages lists none", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			got, err := h.Port.ListWikiPages(ctx, Owner, Repo, 1, 30)
			must(t, err)
			if got.Pages == nil || len(got.Pages) != 0 {
				t.Fatalf("want an empty non-nil list, got %#v", got.Pages)
			}
		}},
		{"a created page reads back by the name it answered", func(t *testing.T, h Harness) {
			made := newWikiPage(t, h, "My Page", "# hello\n")
			if made.Title != "My Page" || made.SubURL == "" || made.LastCommit.SHA == "" {
				t.Fatalf("create answered %+v", made)
			}
			got, err := h.Port.GetWikiPage(ctx, Owner, Repo, made.SubURL)
			must(t, err)
			if got.Title != "My Page" || got.ContentBase64 != b64("# hello\n") || got.SubURL != made.SubURL {
				t.Fatalf("read back %+v", got)
			}
			if got.LastCommit.SHA != made.LastCommit.SHA {
				t.Fatalf("last commit %q, create said %q", got.LastCommit.SHA, made.LastCommit.SHA)
			}
		}},
		{"a get of an unknown page is not found", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			_, err := h.Port.GetWikiPage(ctx, Owner, Repo, "Missing")
			wantNotFound(t, err)
		}},
		{"a slash in a title is part of one page name", func(t *testing.T, h Harness) {
			made := newWikiPage(t, h, "Guides/Setup", "setup")
			for _, name := range []string{made.SubURL, "Guides/Setup", "Guides%2FSetup"} {
				got, err := h.Port.GetWikiPage(ctx, Owner, Repo, name)
				must(t, err)
				if got.Title != "Guides/Setup" {
					t.Fatalf("reading %q gave %+v", name, got)
				}
			}
			_, err := h.Port.GetWikiPage(ctx, Owner, Repo, "Guides")
			wantNotFound(t, err)
		}},
		{"creating an existing title overwrites it", func(t *testing.T, h Harness) {
			first := newWikiPage(t, h, "Home", "one")
			_, err := h.Port.CreateWikiPage(ctx, Owner, Repo, "Home", b64("two"), "again")
			must(t, err)
			got, err := h.Port.GetWikiPage(ctx, Owner, Repo, first.SubURL)
			must(t, err)
			if got.ContentBase64 != b64("two") {
				t.Fatalf("content %q, want the second write", got.ContentBase64)
			}
			list, err := h.Port.ListWikiPages(ctx, Owner, Repo, 1, 30)
			must(t, err)
			if len(list.Pages) != 1 {
				t.Fatalf("want one page, got %+v", list.Pages)
			}
		}},
		{"an edit changes content and records a revision", func(t *testing.T, h Harness) {
			made := newWikiPage(t, h, "Home", "one")
			edited, err := h.Port.EditWikiPage(ctx, Owner, Repo, made.SubURL, "Home", b64("two"), "edit")
			must(t, err)
			if edited.LastCommit.SHA == "" || edited.LastCommit.SHA == made.LastCommit.SHA {
				t.Fatalf("edit commit %q, create commit %q", edited.LastCommit.SHA, made.LastCommit.SHA)
			}
			got, err := h.Port.GetWikiPage(ctx, Owner, Repo, made.SubURL)
			must(t, err)
			if got.ContentBase64 != b64("two") {
				t.Fatalf("content %q", got.ContentBase64)
			}
			revs, err := h.Port.GetWikiPageRevisions(ctx, Owner, Repo, made.SubURL, 1, 30)
			must(t, err)
			if revs.Count != 2 || len(revs.Commits) != 2 || revs.Commits[0].SHA != edited.LastCommit.SHA || revs.Commits[0].Message != "edit" {
				t.Fatalf("revisions %+v", revs)
			}
		}},
		{"an edit with a new title renames the page", func(t *testing.T, h Harness) {
			made := newWikiPage(t, h, "Old", "body")
			renamed, err := h.Port.EditWikiPage(ctx, Owner, Repo, made.SubURL, "New", b64("body"), "rename")
			must(t, err)
			if renamed.Title != "New" {
				t.Fatalf("renamed %+v", renamed)
			}
			_, err = h.Port.GetWikiPage(ctx, Owner, Repo, made.SubURL)
			wantNotFound(t, err)
			_, err = h.Port.GetWikiPage(ctx, Owner, Repo, renamed.SubURL)
			must(t, err)
		}},
		{"an edit of an unknown page is not found", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			_, err := h.Port.EditWikiPage(ctx, Owner, Repo, "Missing", "Missing", b64("x"), "m")
			wantNotFound(t, err)
		}},
		{"a deleted page is gone", func(t *testing.T, h Harness) {
			made := newWikiPage(t, h, "Home", "one")
			must(t, h.Port.DeleteWikiPage(ctx, Owner, Repo, made.SubURL))
			_, err := h.Port.GetWikiPage(ctx, Owner, Repo, made.SubURL)
			wantNotFound(t, err)
			wantNotFound(t, h.Port.DeleteWikiPage(ctx, Owner, Repo, made.SubURL))
		}},
		{"list paginates and reports the total", func(t *testing.T, h Harness) {
			for i := 0; i < 5; i++ {
				newWikiPage(t, h, fmt.Sprint("Page", i), "x")
			}
			page := func(p int) forge.WikiPageList {
				got, err := h.Port.ListWikiPages(ctx, Owner, Repo, p, 2)
				must(t, err)
				return got
			}
			p1, p2, p3, p4 := page(1), page(2), page(3), page(4)
			if len(p1.Pages) != 2 || len(p2.Pages) != 2 || len(p3.Pages) != 1 || len(p4.Pages) != 0 {
				t.Fatalf("page sizes %d %d %d %d", len(p1.Pages), len(p2.Pages), len(p3.Pages), len(p4.Pages))
			}
			if p1.Pages[0].SubURL == p2.Pages[0].SubURL {
				t.Fatal("pages must not overlap")
			}
			if p1.TotalCount == nil || *p1.TotalCount != 5 {
				t.Fatalf("total count %v, want 5", p1.TotalCount)
			}
		}},
		{"revisions paginate newest first and an unknown page is not found", func(t *testing.T, h Harness) {
			made := newWikiPage(t, h, "Home", "v1")
			for i := 2; i <= 3; i++ {
				_, err := h.Port.EditWikiPage(ctx, Owner, Repo, made.SubURL, "Home", b64(fmt.Sprint("v", i)), fmt.Sprint("edit ", i))
				must(t, err)
			}
			p1, err := h.Port.GetWikiPageRevisions(ctx, Owner, Repo, made.SubURL, 1, 2)
			must(t, err)
			p2, err := h.Port.GetWikiPageRevisions(ctx, Owner, Repo, made.SubURL, 2, 2)
			must(t, err)
			if p1.Count != 3 || p2.Count != 3 || len(p1.Commits) != 2 || len(p2.Commits) != 1 {
				t.Fatalf("p1=%+v p2=%+v", p1, p2)
			}
			if p1.Commits[0].Message != "edit 3" || p2.Commits[0].Message != "create Home" {
				t.Fatalf("order: p1=%+v p2=%+v", p1.Commits, p2.Commits)
			}
			_, err = h.Port.GetWikiPageRevisions(ctx, Owner, Repo, "Missing", 1, 30)
			wantNotFound(t, err)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.run(t, newHarness(t))
		})
	}
}
