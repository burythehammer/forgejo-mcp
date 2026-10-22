// SPDX-License-Identifier: GPL-3.0-or-later

package forge

import "context"

// MaxInlineDownloadBytes caps inline base64 attachment payloads.
// Files at or above this size return metadata only; the caller is expected
// to fetch browser_download_url directly with the same auth token.
// See docs/plans/issue-attachments.md and Codeberg issue #106.
const MaxInlineDownloadBytes = 1 * 1024 * 1024

// MaxFileSourceBytes bounds the bytes read from Forgejo for one repository
// file, so a file read never holds an unbounded body in memory. A file over it
// is refused with ErrPayloadTooLarge. It is a read bound: the text a caller
// receives is bounded again by the use case (app/repo.InlineFileBytes).
const MaxFileSourceBytes = 8 * 1024 * 1024

// Ceilings is the instance's pagination ceiling, which every list use case
// reads through its port per call. *fake.Forgejo, the SDK adapter and the
// composed Forgejo port all satisfy it.
type Ceilings interface {
	// MaxResponseItems is the instance's pagination ceiling; ok is false when
	// it cannot be determined, which is "unknown", never a ceiling of zero.
	MaxResponseItems(ctx context.Context) (limit int, ok bool)
}
