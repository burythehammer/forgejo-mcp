// SPDX-License-Identifier: GPL-3.0-or-later

package model

import "time"

// Release mirrors the SDK's release wire shape field for field.
type Release struct {
	ID           int64         `json:"id"`
	TagName      string        `json:"tag_name"`
	Target       string        `json:"target_commitish"`
	Title        string        `json:"name"`
	Note         string        `json:"body"`
	URL          string        `json:"url"`
	HTMLURL      string        `json:"html_url"`
	TarURL       string        `json:"tarball_url"`
	ZipURL       string        `json:"zipball_url"`
	IsDraft      bool          `json:"draft"`
	IsPrerelease bool          `json:"prerelease"`
	CreatedAt    time.Time     `json:"created_at"`
	PublishedAt  time.Time     `json:"published_at"`
	Publisher    *User         `json:"author"`
	Attachments  []*Attachment `json:"assets"`
}
