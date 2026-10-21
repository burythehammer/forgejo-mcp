// SPDX-License-Identifier: GPL-3.0-or-later

package forge

import (
	"context"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// Releases is the release domain of the port: releases and the assets
// (attachments) hung off them.
//
// Failures from Forgejo are *StatusError. Uploads and downloads of attachment
// bytes are described with Upload and Download (see attachment.go).
type Releases interface {
	ListReleases(ctx context.Context, owner, repo string, opt forgejo_sdk.ListReleasesOptions) ([]*forgejo_sdk.Release, error)
	GetRelease(ctx context.Context, owner, repo string, id int64) (*forgejo_sdk.Release, error)
	GetReleaseByTag(ctx context.Context, owner, repo, tag string) (*forgejo_sdk.Release, error)
	// GetLatestRelease returns the newest release that is neither a draft nor
	// a prerelease.
	GetLatestRelease(ctx context.Context, owner, repo string) (*forgejo_sdk.Release, error)
	CreateRelease(ctx context.Context, owner, repo string, opt forgejo_sdk.CreateReleaseOption) (*forgejo_sdk.Release, error)
	// EditRelease changes the fields opt supplies: empty strings and nil
	// booleans leave the release as it was.
	EditRelease(ctx context.Context, owner, repo string, id int64, opt forgejo_sdk.EditReleaseOption) (*forgejo_sdk.Release, error)
	DeleteRelease(ctx context.Context, owner, repo string, id int64) error
	DeleteReleaseByTag(ctx context.Context, owner, repo, tag string) error

	// ListReleaseAttachments returns every attachment: Forgejo does not
	// paginate this endpoint.
	ListReleaseAttachments(ctx context.Context, owner, repo string, releaseID int64) ([]*forgejo_sdk.Attachment, error)
	GetReleaseAttachment(ctx context.Context, owner, repo string, releaseID, id int64) (*forgejo_sdk.Attachment, error)
	CreateReleaseAttachment(ctx context.Context, owner, repo string, releaseID int64, up Upload) (*forgejo_sdk.Attachment, error)
	EditReleaseAttachment(ctx context.Context, owner, repo string, releaseID, id int64, name string) (*forgejo_sdk.Attachment, error)
	DeleteReleaseAttachment(ctx context.Context, owner, repo string, releaseID, id int64) error
}
