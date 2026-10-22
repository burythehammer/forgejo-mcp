// SPDX-License-Identifier: GPL-3.0-or-later

package forgetest

import (
	"errors"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// attachmentScope is one parent an attachment can hang off, seen through the
// port: the same cases run for issues and for issue comments.
type attachmentScope struct {
	name string
	seed func(h Harness) // makes the parent (id parentID) exist
	list func(h Harness) ([]*forgejo_sdk.Attachment, error)
	get  func(h Harness, id int64) (*forgejo_sdk.Attachment, error)
	// create uploads to the parent.
	create func(h Harness, up forge.Upload) (*forgejo_sdk.Attachment, error)
	edit   func(h Harness, id int64, name string) (*forgejo_sdk.Attachment, error)
	del    func(h Harness, id int64) error
	// unknown lists the attachments of a parent that was never seeded.
	unknown func(h Harness) error
}

const parentID = 7

func attachmentScopes() []attachmentScope {
	return []attachmentScope{
		{
			name: "issue",
			seed: func(h Harness) { h.Seed.AddRepo(Owner, Repo); h.Seed.AddAttachmentIssue(Owner, Repo, parentID) },
			list: func(h Harness) ([]*forgejo_sdk.Attachment, error) {
				return h.Port.ListIssueAttachments(ctx, Owner, Repo, parentID)
			},
			get: func(h Harness, id int64) (*forgejo_sdk.Attachment, error) {
				return h.Port.GetIssueAttachment(ctx, Owner, Repo, parentID, id)
			},
			create: func(h Harness, up forge.Upload) (*forgejo_sdk.Attachment, error) {
				return h.Port.CreateIssueAttachment(ctx, Owner, Repo, parentID, up)
			},
			edit: func(h Harness, id int64, name string) (*forgejo_sdk.Attachment, error) {
				return h.Port.EditIssueAttachment(ctx, Owner, Repo, parentID, id, name)
			},
			del:     func(h Harness, id int64) error { return h.Port.DeleteIssueAttachment(ctx, Owner, Repo, parentID, id) },
			unknown: func(h Harness) error { _, err := h.Port.ListIssueAttachments(ctx, Owner, Repo, 99); return err },
		},
		{
			name: "comment",
			seed: func(h Harness) { h.Seed.AddRepo(Owner, Repo); h.Seed.AddAttachmentComment(Owner, Repo, parentID) },
			list: func(h Harness) ([]*forgejo_sdk.Attachment, error) {
				return h.Port.ListCommentAttachments(ctx, Owner, Repo, parentID)
			},
			get: func(h Harness, id int64) (*forgejo_sdk.Attachment, error) {
				return h.Port.GetCommentAttachment(ctx, Owner, Repo, parentID, id)
			},
			create: func(h Harness, up forge.Upload) (*forgejo_sdk.Attachment, error) {
				return h.Port.CreateCommentAttachment(ctx, Owner, Repo, parentID, up)
			},
			edit: func(h Harness, id int64, name string) (*forgejo_sdk.Attachment, error) {
				return h.Port.EditCommentAttachment(ctx, Owner, Repo, parentID, id, name)
			},
			del:     func(h Harness, id int64) error { return h.Port.DeleteCommentAttachment(ctx, Owner, Repo, parentID, id) },
			unknown: func(h Harness) error { _, err := h.Port.ListCommentAttachments(ctx, Owner, Repo, 99); return err },
		},
	}
}

// RunAttachments runs the issue and comment attachment contract against the
// implementation that newHarness builds, once per case.
func RunAttachments(t *testing.T, newHarness func(t *testing.T) Harness) {
	for _, sc := range attachmentScopes() {
		cases := []struct {
			name string
			run  func(t *testing.T, h Harness)
		}{
			{"upload, list, get, rename, download and delete", func(t *testing.T, h Harness) {
				sc.seed(h)
				att, err := sc.create(h, upload("shot.png", "image/png", "pixels"))
				must(t, err)
				if att.Name != "shot.png" || att.Size != int64(len("pixels")) || att.DownloadURL == "" {
					t.Fatalf("got %+v", att)
				}
				list, err := sc.list(h)
				must(t, err)
				if len(list) != 1 || list[0].ID != att.ID {
					t.Fatalf("list = %+v", list)
				}
				got, err := sc.get(h, att.ID)
				must(t, err)
				if got.Name != "shot.png" {
					t.Fatalf("got %+v", got)
				}
				renamed, err := sc.edit(h, att.ID, "renamed.png")
				must(t, err)
				if renamed.Name != "renamed.png" {
					t.Fatalf("renamed = %+v", renamed)
				}
				dl, err := h.Port.DownloadAttachment(ctx, renamed.DownloadURL)
				must(t, err)
				if string(dl.Body) != "pixels" || !strings.HasPrefix(dl.ContentType, "image/png") {
					t.Fatalf("download = %q (%s)", dl.Body, dl.ContentType)
				}
				must(t, sc.del(h, att.ID))
				_, err = sc.get(h, att.ID)
				wantNotFound(t, err)
				list, err = sc.list(h)
				must(t, err)
				if len(list) != 0 {
					t.Fatalf("list after delete = %+v", list)
				}
			}},
			{"a parent with no attachments lists none", func(t *testing.T, h Harness) {
				sc.seed(h)
				list, err := sc.list(h)
				must(t, err)
				if len(list) != 0 {
					t.Fatalf("list = %+v", list)
				}
			}},
			{"unknown parent and attachment are not found", func(t *testing.T, h Harness) {
				wantNotFound(t, sc.unknown(h))
				sc.seed(h)
				_, err := sc.get(h, 99)
				wantNotFound(t, err)
				_, err = sc.edit(h, 99, "n")
				wantNotFound(t, err)
				wantNotFound(t, sc.del(h, 99))
			}},
			{"an upload without a filename is rejected", func(t *testing.T, h Harness) {
				sc.seed(h)
				if _, err := sc.create(h, upload("", "", "x")); err == nil {
					t.Fatal("want an error")
				}
			}},
		}
		for _, tc := range cases {
			t.Run(sc.name+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				tc.run(t, newHarness(t))
			})
		}
	}

	t.Run("download from another origin is refused", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		_, err := h.Port.DownloadAttachment(ctx, "https://elsewhere.example/attachments/x")
		if !errors.Is(err, forge.ErrForeignOrigin) {
			t.Fatalf("got %v, want ErrForeignOrigin", err)
		}
	})

	t.Run("download over the inline cap is refused", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		sc := attachmentScopes()[0]
		sc.seed(h)
		att, err := sc.create(h, upload("big.bin", "", strings.Repeat("x", int(forge.MaxInlineDownloadBytes)+1)))
		must(t, err)
		_, err = h.Port.DownloadAttachment(ctx, att.DownloadURL)
		if !errors.Is(err, forge.ErrPayloadTooLarge) {
			t.Fatalf("got %v, want ErrPayloadTooLarge", err)
		}
	})
}
