// SPDX-License-Identifier: GPL-3.0-or-later

// Package releaseadapter implements the release use cases' port (app/release.Port)
// over the Forgejo port (forge.Releases and forge.Attachments). It is the one
// place that translates between repo-owned types and SDK types for the release
// domain: inputs become SDK options, and SDK results become app/model values
// through their JSON form, which is exactly what a tool renders.
package releaseadapter

import (
	"context"
	"errors"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/download"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/model"
	apprelease "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/release"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge/bridge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// ReleaseSource is the part of the Forgejo port the release domain reads
// releases through: its release methods and the instance's pagination ceiling.
type ReleaseSource interface {
	forge.Releases
	forge.Ceilings
}

// New returns the use cases' port over the release and attachment ports.
func New(rel ReleaseSource, att forge.Attachments) apprelease.Port {
	return adapter{rel: rel, att: att}
}

type adapter struct {
	rel ReleaseSource
	att forge.Attachments
}

// MaxResponseItems reads the instance's pagination ceiling through the release
// source.
func (a adapter) MaxResponseItems(ctx context.Context) (int, bool) {
	return a.rel.MaxResponseItems(ctx)
}

var _ apprelease.Port = adapter{}

func (a adapter) ListReleases(ctx context.Context, owner, repo string, p apprelease.Paging) ([]*model.Release, error) {
	rels, err := a.rel.ListReleases(ctx, owner, repo, forgejo_sdk.ListReleasesOptions{
		ListOptions: forgejo_sdk.ListOptions{Page: p.Page, PageSize: p.Limit},
	})
	return bridge.Convert[[]*model.Release](rels, err)
}

func (a adapter) GetRelease(ctx context.Context, owner, repo string, id int64) (model.Release, error) {
	r, err := a.rel.GetRelease(ctx, owner, repo, id)
	return bridge.Convert[model.Release](r, err)
}

func (a adapter) GetReleaseByTag(ctx context.Context, owner, repo, tag string) (model.Release, error) {
	r, err := a.rel.GetReleaseByTag(ctx, owner, repo, tag)
	return bridge.Convert[model.Release](r, err)
}

func (a adapter) GetLatestRelease(ctx context.Context, owner, repo string) (model.Release, error) {
	r, err := a.rel.GetLatestRelease(ctx, owner, repo)
	return bridge.Convert[model.Release](r, err)
}

func (a adapter) CreateRelease(ctx context.Context, owner, repo string, in apprelease.CreateInput) (model.Release, error) {
	r, err := a.rel.CreateRelease(ctx, owner, repo, forgejo_sdk.CreateReleaseOption{
		TagName: in.TagName, Target: in.Target, Title: in.Title, Note: in.Note,
		IsDraft: in.IsDraft, IsPrerelease: in.IsPrerelease,
	})
	return bridge.Convert[model.Release](r, err)
}

func (a adapter) EditRelease(ctx context.Context, owner, repo string, id int64, in apprelease.EditInput) (model.Release, error) {
	r, err := a.rel.EditRelease(ctx, owner, repo, id, forgejo_sdk.EditReleaseOption{
		TagName: in.TagName, Target: in.Target, Title: in.Title, Note: in.Note,
		IsDraft: in.IsDraft, IsPrerelease: in.IsPrerelease,
	})
	return bridge.Convert[model.Release](r, err)
}

func (a adapter) DeleteRelease(ctx context.Context, owner, repo string, id int64) error {
	return a.rel.DeleteRelease(ctx, owner, repo, id)
}

func (a adapter) DeleteReleaseByTag(ctx context.Context, owner, repo, tag string) error {
	return a.rel.DeleteReleaseByTag(ctx, owner, repo, tag)
}

func (a adapter) ListAttachments(ctx context.Context, owner, repo string, releaseID int64) ([]*model.Attachment, error) {
	all, err := a.rel.ListReleaseAttachments(ctx, owner, repo, releaseID)
	return bridge.Convert[[]*model.Attachment](all, err)
}

func (a adapter) GetAttachment(ctx context.Context, owner, repo string, releaseID, id int64) (model.Attachment, error) {
	x, err := a.rel.GetReleaseAttachment(ctx, owner, repo, releaseID, id)
	return bridge.Convert[model.Attachment](x, err)
}

func (a adapter) CreateAttachment(ctx context.Context, owner, repo string, releaseID int64, up apprelease.Upload) (model.Attachment, error) {
	x, err := a.rel.CreateReleaseAttachment(ctx, owner, repo, releaseID, forge.Upload{Filename: up.Filename, MIMEType: up.MIMEType, Content: up.Content})
	return bridge.Convert[model.Attachment](x, err)
}

func (a adapter) EditAttachment(ctx context.Context, owner, repo string, releaseID, id int64, name string) (model.Attachment, error) {
	x, err := a.rel.EditReleaseAttachment(ctx, owner, repo, releaseID, id, name)
	return bridge.Convert[model.Attachment](x, err)
}

func (a adapter) DeleteAttachment(ctx context.Context, owner, repo string, releaseID, id int64) error {
	return a.rel.DeleteReleaseAttachment(ctx, owner, repo, releaseID, id)
}

// DownloadAttachment fetches an asset's bytes. The forge's inline-cap sentinel
// is re-expressed as the use case's, so the use case never imports pkg/forge.
func (a adapter) DownloadAttachment(ctx context.Context, url string) (model.AttachmentBody, error) {
	d, err := a.att.DownloadAttachment(ctx, url)
	if errors.Is(err, forge.ErrPayloadTooLarge) {
		return model.AttachmentBody{}, download.ErrPayloadTooLarge
	}
	if err != nil {
		return model.AttachmentBody{}, err
	}
	return model.AttachmentBody{Body: d.Body, ContentType: d.ContentType}, nil
}
