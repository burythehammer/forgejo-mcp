// SPDX-License-Identifier: GPL-3.0-or-later

package model

import "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/list"

// ReleasePage is the response of list_releases: one page of releases with the
// continuation signal of docs/design/output-bounding.md.
type ReleasePage struct {
	Releases []*Release `json:"releases"`
	PageMeta
}

// AttachmentPage is the response of list_release_attachments.
type AttachmentPage struct {
	Attachments []*Attachment `json:"attachments"`
	PageMeta
}

// AttachmentPageOf is the one place a list of attachments becomes its response.
// Forgejo reports no total for an attachment listing, so the wire form carries
// none, even though the whole list is read to page it.
func AttachmentPageOf(b list.Bounded[*Attachment]) AttachmentPage {
	meta := PageMetaOf(b)
	meta.TotalCount = nil
	return AttachmentPage{Attachments: b.Items, PageMeta: meta}
}
