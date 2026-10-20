// SPDX-License-Identifier: GPL-3.0-or-later

// Package attachment holds the issue and issue-comment attachment use cases
// behind the attachment tools. Like all of app/ it is MCP-free and SDK-free
// (scripts/ci/check-app-boundary.sh): use cases take typed inputs, reach
// Forgejo only through Port, and return repo-owned types from app/model for an
// MCP adapter in operation/attachment to render.
package attachment

import (
	"context"
	"errors"
	"io"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/model"
)

// Scope says which parent an attachment hangs off: an issue (by its index) or
// a comment (by its ID).
type Scope int

const (
	ScopeIssue Scope = iota
	ScopeComment
)

// label is the word the tools use for the scope in their messages.
func (s Scope) label() string {
	if s == ScopeComment {
		return "comment"
	}
	return "issue"
}

// ErrNotFound is what Port.List returns for a parent Forgejo does not know. A
// list treats it as an empty list: Forgejo answers 404 for a parent with no
// attachments as often as for a missing one.
var ErrNotFound = errors.New("attachment parent not found")

// Port is the attachment domain as the use cases need it, in repo-owned types.
// pkg/forge/attachmentadapter implements it over forge.Attachments.
type Port interface {
	List(ctx context.Context, owner, repo string, s Scope, parent int64) ([]*model.Attachment, error)
	Get(ctx context.Context, owner, repo string, s Scope, parent, id int64) (model.Attachment, error)
	Create(ctx context.Context, owner, repo string, s Scope, parent int64, up Upload) (model.Attachment, error)
	Edit(ctx context.Context, owner, repo string, s Scope, parent, id int64, name string) (model.Attachment, error)
	Delete(ctx context.Context, owner, repo string, s Scope, parent, id int64) error
	// Download fetches an attachment's bytes; a body over the inline cap is
	// download.ErrPayloadTooLarge.
	Download(ctx context.Context, url string) (model.AttachmentBody, error)
}

// Upload is one file to attach.
type Upload struct {
	Filename string
	MIMEType string
	Content  io.Reader
}
