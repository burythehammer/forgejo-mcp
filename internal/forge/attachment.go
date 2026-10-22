// SPDX-License-Identifier: GPL-3.0-or-later

package forge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// ErrPayloadTooLarge is returned by DownloadAttachment when the body exceeds
// the inline cap (forgejo.MaxInlineDownloadBytes).
var ErrPayloadTooLarge = errors.New("payload exceeds inline cap")

// ErrForeignOrigin is returned by DownloadAttachment when a download URL is not
// on the configured Forgejo origin. The credential is never sent to such a URL.
var ErrForeignOrigin = errors.New("download URL is not on the configured Forgejo origin")

// CheckOrigin returns nil when raw may carry the credential: a relative
// reference, or an absolute URL whose scheme, host and port (defaulted from the
// scheme) equal base's. Anything else wraps ErrForeignOrigin and names the host
// but not the full URL, which may carry a signature.
func CheckOrigin(base, raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%w: the URL does not parse", ErrForeignOrigin)
	}
	if !u.IsAbs() {
		return nil
	}
	b, err := url.Parse(strings.TrimRight(base, "/"))
	if err != nil || !b.IsAbs() {
		return fmt.Errorf("%w: the configured Forgejo URL is not absolute", ErrForeignOrigin)
	}
	if originKey(u) != originKey(b) {
		return fmt.Errorf("%w: host %s", ErrForeignOrigin, u.Host)
	}
	return nil
}

// originKey is scheme, lowercased host and explicit-or-default port.
func originKey(u *url.URL) string {
	scheme := strings.ToLower(u.Scheme)
	port := u.Port()
	if port == "" {
		switch scheme {
		case "https":
			port = "443"
		case "http":
			port = "80"
		}
	}
	return scheme + "://" + strings.ToLower(u.Hostname()) + ":" + port
}

// Upload is one file to send as an attachment. Content is streamed and not
// closed by the port; MIMEType may be empty (application/octet-stream).
type Upload struct {
	Filename string
	MIMEType string
	Content  io.Reader
}

// Download is the bytes of an attachment and the content type Forgejo served.
type Download struct {
	Body        []byte
	ContentType string
}

// Attachments is the issue and issue-comment attachment domain of the port.
// (Release attachments belong to Releases; DownloadAttachment serves both.)
//
// Failures from Forgejo are *StatusError, so an unknown issue, comment or
// attachment is forge.IsNotFound. Uploads honour ctx: cancelling it abandons
// the upload.
type Attachments interface {
	ListIssueAttachments(ctx context.Context, owner, repo string, index int64) ([]*forgejo_sdk.Attachment, error)
	GetIssueAttachment(ctx context.Context, owner, repo string, index, id int64) (*forgejo_sdk.Attachment, error)
	CreateIssueAttachment(ctx context.Context, owner, repo string, index int64, up Upload) (*forgejo_sdk.Attachment, error)
	EditIssueAttachment(ctx context.Context, owner, repo string, index, id int64, name string) (*forgejo_sdk.Attachment, error)
	DeleteIssueAttachment(ctx context.Context, owner, repo string, index, id int64) error

	ListCommentAttachments(ctx context.Context, owner, repo string, commentID int64) ([]*forgejo_sdk.Attachment, error)
	GetCommentAttachment(ctx context.Context, owner, repo string, commentID, id int64) (*forgejo_sdk.Attachment, error)
	CreateCommentAttachment(ctx context.Context, owner, repo string, commentID int64, up Upload) (*forgejo_sdk.Attachment, error)
	EditCommentAttachment(ctx context.Context, owner, repo string, commentID, id int64, name string) (*forgejo_sdk.Attachment, error)
	DeleteCommentAttachment(ctx context.Context, owner, repo string, commentID, id int64) error

	// DownloadAttachment fetches the bytes behind an attachment's
	// browser_download_url, up to the inline cap; a larger body is
	// ErrPayloadTooLarge.
	DownloadAttachment(ctx context.Context, downloadURL string) (Download, error)
}
