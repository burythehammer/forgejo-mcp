// SPDX-License-Identifier: GPL-3.0-or-later

// Package download holds the attachment download decision the release and
// attachment use cases share: whether an attachment's bytes go inline or the
// caller is told where to fetch them. It is MCP-free and SDK-free.
package download

import (
	"context"
	"errors"
	"fmt"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/model"
)

// MaxInlineBytes caps inline attachment payloads. Files at or above this size
// are metadata only; the caller fetches browser_download_url itself.
const MaxInlineBytes = 1 * 1024 * 1024

// ErrPayloadTooLarge is what a fetch returns when the body exceeds MaxInlineBytes.
var ErrPayloadTooLarge = errors.New("payload exceeds inline cap")

// Fetch reads an attachment's bytes from its download URL.
type Fetch func(ctx context.Context, url string) (model.AttachmentBody, error)

// Resolve decides how att is returned. A file at or over the cap, or a body
// that grows past it during the fetch, comes back as metadata with a reason.
// Other fetch failures are returned as errors. When the result is Inline, the
// returned body is the bytes to ship.
func Resolve(ctx context.Context, att *model.Attachment, fetch Fetch) (model.DownloadResult, model.AttachmentBody, error) {
	res := model.DownloadResult{Attachment: att}
	if att.Size >= MaxInlineBytes {
		res.Reason = fmt.Sprintf("size %d bytes >= inline cap %d; fetch browser_download_url with Authorization: token <TOKEN>", att.Size, MaxInlineBytes)
		return res, model.AttachmentBody{}, nil
	}
	body, err := fetch(ctx, att.DownloadURL)
	if err != nil {
		if errors.Is(err, ErrPayloadTooLarge) {
			res.Reason = "body exceeded inline cap during fetch; fetch browser_download_url with Authorization: token <TOKEN>"
			return res, model.AttachmentBody{}, nil
		}
		return res, model.AttachmentBody{}, err
	}
	res.Inline = true
	res.BytesIncluded = int64(len(body.Body))
	return res, body, nil
}
