// SPDX-License-Identifier: GPL-3.0-or-later

package model

// ReleasePage is the response of list_releases: one page of releases with the
// continuation signal of docs/design/output-bounding.md.
type ReleasePage struct {
	Releases []*Release `json:"releases"`
	Page     int        `json:"page"`
	Limit    int        `json:"limit"`
	Count    int        `json:"count"`
	HasNext  bool       `json:"has_next"`
}

// AttachmentPage is the response of list_release_attachments.
type AttachmentPage struct {
	Attachments []*Attachment `json:"attachments"`
	Page        int           `json:"page"`
	Limit       int           `json:"limit"`
	Count       int           `json:"count"`
	HasNext     bool          `json:"has_next"`
}
