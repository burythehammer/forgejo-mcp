// SPDX-License-Identifier: GPL-3.0-or-later

package forgetest

import (
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// RunReleases runs the release contract (releases and their assets) against
// the implementation that newHarness builds, once per case.
func RunReleases(t *testing.T, newHarness func(t *testing.T) Harness) {
	cases := []struct {
		name string
		run  func(t *testing.T, h Harness)
	}{
		{"unknown repository and release are not found", func(t *testing.T, h Harness) {
			_, err := h.Port.GetRelease(ctx, "nobody", "nothing", 1)
			wantNotFound(t, err)
			h.Seed.AddRepo(Owner, Repo)
			_, err = h.Port.GetRelease(ctx, Owner, Repo, 99)
			wantNotFound(t, err)
			_, err = h.Port.GetReleaseByTag(ctx, Owner, Repo, "v9")
			wantNotFound(t, err)
			wantNotFound(t, h.Port.DeleteRelease(ctx, Owner, Repo, 99))
			wantNotFound(t, h.Port.DeleteReleaseByTag(ctx, Owner, Repo, "v9"))
			_, err = h.Port.CreateRelease(ctx, "nobody", "nothing", forgejo_sdk.CreateReleaseOption{TagName: "v1", Title: "v1"})
			wantNotFound(t, err)
		}},
		{"created release reads back by id and by tag", func(t *testing.T, h Harness) {
			a := newRelease(t, h, "v1", false, false)
			b := newRelease(t, h, "v2", false, false)
			if a.ID == b.ID {
				t.Fatalf("ids must differ, both %d", a.ID)
			}
			got, err := h.Port.GetRelease(ctx, Owner, Repo, a.ID)
			must(t, err)
			if got.TagName != "v1" || got.Title != "title v1" || got.Note != "notes v1" || got.IsDraft || got.IsPrerelease {
				t.Fatalf("got %+v", got)
			}
			byTag, err := h.Port.GetReleaseByTag(ctx, Owner, Repo, "v2")
			must(t, err)
			if byTag.ID != b.ID {
				t.Fatalf("by tag got id %d, want %d", byTag.ID, b.ID)
			}
		}},
		{"create rejects a duplicate tag and a missing title", func(t *testing.T, h Harness) {
			newRelease(t, h, "v1", false, false)
			_, err := h.Port.CreateRelease(ctx, Owner, Repo, forgejo_sdk.CreateReleaseOption{TagName: "v1", Title: "again"})
			if err == nil {
				t.Fatal("want an error for a duplicate tag")
			}
			_, err = h.Port.CreateRelease(ctx, Owner, Repo, forgejo_sdk.CreateReleaseOption{TagName: "v3"})
			if err == nil {
				t.Fatal("want an error for a missing title")
			}
		}},
		{"list is newest first and paginates", func(t *testing.T, h Harness) {
			for _, tag := range []string{"v1", "v2", "v3"} {
				newRelease(t, h, tag, false, false)
			}
			all, err := h.Port.ListReleases(ctx, Owner, Repo, forgejo_sdk.ListReleasesOptions{})
			must(t, err)
			if got := tags(all); got != "v3,v2,v1" {
				t.Fatalf("got %s", got)
			}
			p2, err := h.Port.ListReleases(ctx, Owner, Repo, forgejo_sdk.ListReleasesOptions{ListOptions: forgejo_sdk.ListOptions{Page: 2, PageSize: 2}})
			must(t, err)
			if got := tags(p2); got != "v1" {
				t.Fatalf("page 2 got %s", got)
			}
		}},
		{"list filters drafts and prereleases", func(t *testing.T, h Harness) {
			newRelease(t, h, "v1", false, false)
			newRelease(t, h, "v2-draft", true, false)
			newRelease(t, h, "v3-rc", false, true)
			yes, no := true, false
			drafts, err := h.Port.ListReleases(ctx, Owner, Repo, forgejo_sdk.ListReleasesOptions{IsDraft: &yes})
			must(t, err)
			if got := tags(drafts); got != "v2-draft" {
				t.Fatalf("drafts got %s", got)
			}
			pre, err := h.Port.ListReleases(ctx, Owner, Repo, forgejo_sdk.ListReleasesOptions{IsPreRelease: &yes})
			must(t, err)
			if got := tags(pre); got != "v3-rc" {
				t.Fatalf("prereleases got %s", got)
			}
			final, err := h.Port.ListReleases(ctx, Owner, Repo, forgejo_sdk.ListReleasesOptions{IsDraft: &no, IsPreRelease: &no})
			must(t, err)
			if got := tags(final); got != "v1" {
				t.Fatalf("published got %s", got)
			}
		}},
		{"latest skips drafts and prereleases", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			_, err := h.Port.GetLatestRelease(ctx, Owner, Repo)
			wantNotFound(t, err)
			newRelease(t, h, "v1", false, false)
			newRelease(t, h, "v2-draft", true, false)
			newRelease(t, h, "v3-rc", false, true)
			got, err := h.Port.GetLatestRelease(ctx, Owner, Repo)
			must(t, err)
			if got.TagName != "v1" {
				t.Fatalf("latest = %s, want v1", got.TagName)
			}
		}},
		{"edit changes only what it supplies", func(t *testing.T, h Harness) {
			rel := newRelease(t, h, "v1", false, false)
			yes := true
			got, err := h.Port.EditRelease(ctx, Owner, Repo, rel.ID, forgejo_sdk.EditReleaseOption{Note: "new notes", IsPrerelease: &yes})
			must(t, err)
			if got.Note != "new notes" || !got.IsPrerelease || got.Title != "title v1" || got.TagName != "v1" || got.IsDraft {
				t.Fatalf("got %+v", got)
			}
			again, err := h.Port.GetRelease(ctx, Owner, Repo, rel.ID)
			must(t, err)
			if again.Note != "new notes" || !again.IsPrerelease {
				t.Fatalf("not persisted: %+v", again)
			}
			_, err = h.Port.EditRelease(ctx, Owner, Repo, 99, forgejo_sdk.EditReleaseOption{Note: "x"})
			wantNotFound(t, err)
		}},
		{"delete by id and by tag", func(t *testing.T, h Harness) {
			a := newRelease(t, h, "v1", false, false)
			newRelease(t, h, "v2", false, false)
			must(t, h.Port.DeleteRelease(ctx, Owner, Repo, a.ID))
			_, err := h.Port.GetRelease(ctx, Owner, Repo, a.ID)
			wantNotFound(t, err)
			must(t, h.Port.DeleteReleaseByTag(ctx, Owner, Repo, "v2"))
			_, err = h.Port.GetReleaseByTag(ctx, Owner, Repo, "v2")
			wantNotFound(t, err)
		}},
		{"attachments upload, list, get, rename, download and delete", func(t *testing.T, h Harness) {
			rel := newRelease(t, h, "v1", false, false)
			att, err := h.Port.CreateReleaseAttachment(ctx, Owner, Repo, rel.ID, upload("app.bin", "application/zip", "binary-bytes"))
			must(t, err)
			if att.Name != "app.bin" || att.Size != int64(len("binary-bytes")) || att.DownloadURL == "" {
				t.Fatalf("got %+v", att)
			}
			list, err := h.Port.ListReleaseAttachments(ctx, Owner, Repo, rel.ID)
			must(t, err)
			if len(list) != 1 || list[0].ID != att.ID {
				t.Fatalf("list = %+v", list)
			}
			got, err := h.Port.GetReleaseAttachment(ctx, Owner, Repo, rel.ID, att.ID)
			must(t, err)
			if got.Name != "app.bin" {
				t.Fatalf("got %+v", got)
			}
			renamed, err := h.Port.EditReleaseAttachment(ctx, Owner, Repo, rel.ID, att.ID, "app-v1.bin")
			must(t, err)
			if renamed.Name != "app-v1.bin" {
				t.Fatalf("renamed = %+v", renamed)
			}
			dl, err := h.Port.DownloadAttachment(ctx, renamed.DownloadURL)
			must(t, err)
			if string(dl.Body) != "binary-bytes" || !strings.HasPrefix(dl.ContentType, "application/zip") {
				t.Fatalf("download = %q (%s)", dl.Body, dl.ContentType)
			}
			must(t, h.Port.DeleteReleaseAttachment(ctx, Owner, Repo, rel.ID, att.ID))
			_, err = h.Port.GetReleaseAttachment(ctx, Owner, Repo, rel.ID, att.ID)
			wantNotFound(t, err)
			list, err = h.Port.ListReleaseAttachments(ctx, Owner, Repo, rel.ID)
			must(t, err)
			if len(list) != 0 {
				t.Fatalf("list after delete = %+v", list)
			}
		}},
		{"attachments of an unknown release are not found", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			_, err := h.Port.ListReleaseAttachments(ctx, Owner, Repo, 99)
			wantNotFound(t, err)
			_, err = h.Port.CreateReleaseAttachment(ctx, Owner, Repo, 99, upload("a", "", "x"))
			wantNotFound(t, err)
			rel := newRelease(t, h, "v1", false, false)
			_, err = h.Port.GetReleaseAttachment(ctx, Owner, Repo, rel.ID, 99)
			wantNotFound(t, err)
			_, err = h.Port.EditReleaseAttachment(ctx, Owner, Repo, rel.ID, 99, "n")
			wantNotFound(t, err)
			wantNotFound(t, h.Port.DeleteReleaseAttachment(ctx, Owner, Repo, rel.ID, 99))
		}},
		{"a release carries its attachments", func(t *testing.T, h Harness) {
			rel := newRelease(t, h, "v1", false, false)
			_, err := h.Port.CreateReleaseAttachment(ctx, Owner, Repo, rel.ID, upload("a.txt", "", "x"))
			must(t, err)
			got, err := h.Port.GetRelease(ctx, Owner, Repo, rel.ID)
			must(t, err)
			if len(got.Attachments) != 1 || got.Attachments[0].Name != "a.txt" {
				t.Fatalf("attachments = %+v", got.Attachments)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.run(t, newHarness(t))
		})
	}
}

// newRelease creates a release in Owner/Repo, making the repository first. Its
// title is "title <tag>" and its notes "notes <tag>".
func newRelease(t *testing.T, h Harness, tag string, draft, prerelease bool) *forgejo_sdk.Release {
	t.Helper()
	h.Seed.AddRepo(Owner, Repo)
	rel, err := h.Port.CreateRelease(ctx, Owner, Repo, forgejo_sdk.CreateReleaseOption{
		TagName: tag, Target: "main", Title: "title " + tag, Note: "notes " + tag, IsDraft: draft, IsPrerelease: prerelease,
	})
	must(t, err)
	return rel
}

func tags(rels []*forgejo_sdk.Release) string {
	out := make([]string, 0, len(rels))
	for _, r := range rels {
		out = append(out, r.TagName)
	}
	return strings.Join(out, ",")
}

func upload(name, mimeType, content string) forge.Upload {
	return forge.Upload{Filename: name, MIMEType: mimeType, Content: strings.NewReader(content)}
}
