// SPDX-License-Identifier: GPL-3.0-or-later

package model

import "time"

// Attachment mirrors the SDK's attachment wire shape: the release assets and
// the issue and comment attachments share it.
type Attachment struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	Size          int64     `json:"size"`
	DownloadCount int64     `json:"download_count"`
	Created       time.Time `json:"created_at"`
	UUID          string    `json:"uuid"`
	DownloadURL   string    `json:"browser_download_url"`
}

// AttachmentBody is the bytes of an attachment and the content type Forgejo
// served them with.
type AttachmentBody struct {
	Body        []byte
	ContentType string
}

// DownloadResult is the metadata-only or inline shape a download tool renders.
// Inline is true when the bytes ride along in the tool's embedded resource;
// Reason says why they do not, otherwise.
type DownloadResult struct {
	Attachment    *Attachment `json:"attachment"`
	Inline        bool        `json:"inline"`
	Reason        string      `json:"reason,omitempty"`
	BytesIncluded int64       `json:"bytes_included,omitempty"`
}
