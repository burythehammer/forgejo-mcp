// SPDX-License-Identifier: GPL-3.0-or-later

// Package release holds the release use cases behind the release tools:
// releases and their assets. Like all of internal/app/ it is MCP-free and SDK-free
// (the depguard rules in .golangci.yml): use cases take typed inputs, reach
// Forgejo only through Port, and return repo-owned types from internal/app/model for an
// MCP adapter in internal/operation/release to render.
package release

import (
	"context"
	"io"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/model"
)

// Port is the release domain as the use cases need it, in repo-owned types.
// internal/forge/releaseadapter implements it over forge.Releases and
// forge.Attachments.
type Port interface {
	// MaxResponseItems is the instance's pagination ceiling; ok is false when it
	// cannot be determined, which is "unknown", never a ceiling of zero.
	MaxResponseItems(ctx context.Context) (limit int, ok bool)
	ListReleases(ctx context.Context, owner, repo string, p Paging) ([]*model.Release, error)
	GetRelease(ctx context.Context, owner, repo string, id int64) (model.Release, error)
	GetReleaseByTag(ctx context.Context, owner, repo, tag string) (model.Release, error)
	GetLatestRelease(ctx context.Context, owner, repo string) (model.Release, error)
	CreateRelease(ctx context.Context, owner, repo string, in CreateInput) (model.Release, error)
	EditRelease(ctx context.Context, owner, repo string, id int64, in EditInput) (model.Release, error)
	DeleteRelease(ctx context.Context, owner, repo string, id int64) error
	DeleteReleaseByTag(ctx context.Context, owner, repo, tag string) error

	ListAttachments(ctx context.Context, owner, repo string, releaseID int64) ([]*model.Attachment, error)
	GetAttachment(ctx context.Context, owner, repo string, releaseID, id int64) (model.Attachment, error)
	CreateAttachment(ctx context.Context, owner, repo string, releaseID int64, up Upload) (model.Attachment, error)
	EditAttachment(ctx context.Context, owner, repo string, releaseID, id int64, name string) (model.Attachment, error)
	DeleteAttachment(ctx context.Context, owner, repo string, releaseID, id int64) error
	// DownloadAttachment fetches an asset's bytes; a body over the inline cap is
	// download.ErrPayloadTooLarge.
	DownloadAttachment(ctx context.Context, url string) (model.AttachmentBody, error)
}

// Paging selects a page of a list.
type Paging struct{ Page, Limit int }

// State filters for ListReleases. The empty string is All.
const (
	StateAll        = "all"
	StateDraft      = "draft"
	StatePrerelease = "prerelease"
	StatePublished  = "published"
)

// CreateInput is what creating a release takes. Title defaults to the tag
// name at the use case, since Forgejo rejects an empty one.
type CreateInput struct {
	TagName, Target, Title, Note string
	IsDraft, IsPrerelease        bool
}

// EditInput changes the fields it supplies: empty strings and nil booleans are
// left as they were.
type EditInput struct {
	TagName, Target, Title, Note string
	IsDraft, IsPrerelease        *bool
}

// Upload is one file to attach to a release.
type Upload struct {
	Filename string
	MIMEType string
	Content  io.Reader
}
