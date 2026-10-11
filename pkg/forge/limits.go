// SPDX-License-Identifier: GPL-3.0-or-later

package forge

// MaxInlineDownloadBytes caps inline base64 attachment payloads.
// Files at or above this size return metadata only; the caller is expected
// to fetch browser_download_url directly with the same auth token.
// See docs/plans/issue-attachments.md and Codeberg issue #106.
const MaxInlineDownloadBytes = 1 * 1024 * 1024
