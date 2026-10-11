// SPDX-License-Identifier: GPL-3.0-or-later

// Package releaseadapter implements the release use cases' port (app/release.Port)
// over the Forgejo port (forge.Releases and forge.Attachments). It is the one
// place that translates between repo-owned types and SDK types for the release
// domain: inputs become SDK options, and SDK results become app/model values
// through their JSON form, which is exactly what a tool renders.
package releaseadapter

import (
	"context"
	"encoding/json"
	"errors"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/download"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
	apprelease "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/release"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// New returns the use cases' port over the release and attachment ports.
func New(rel forge.Releases, att forge.Attachments) apprelease.Port {
	return adapter{rel: rel, att: att}
}

type adapter struct {
	rel forge.Releases
	att forge.Attachments
}

var _ apprelease.Port = adapter{}

// convert maps an SDK value onto its repo-owned twin through JSON.
func convert[T any](from any) (T, error) {
	var to T
	b, err := json.Marshal(from)
	if err != nil {
		return to, err
	}
	err = json.Unmarshal(b, &to)
	return to, err
}

func (a adapter) ListReleases(ctx context.Context, owner, repo string, p apprelease.Paging) ([]*model.Release, error) {
	rels, err := a.rel.ListReleases(ctx, owner, repo, forgejo_sdk.ListReleasesOptions{
		ListOptions: forgejo_sdk.ListOptions{Page: p.Page, PageSize: p.Limit},
	})
	if err != nil {
		return nil, err
	}
	return convert[[]*model.Release](rels)
}

func (a adapter) GetRelease(ctx context.Context, owner, repo string, id int64) (model.Release, error) {
	r, err := a.rel.GetRelease(ctx, owner, repo, id)
	if err != nil {
		return model.Release{}, err
	}
	return convert[model.Release](r)
}

func (a adapter) GetReleaseByTag(ctx context.Context, owner, repo, tag string) (model.Release, error) {
	r, err := a.rel.GetReleaseByTag(ctx, owner, repo, tag)
	if err != nil {
		return model.Release{}, err
	}
	return convert[model.Release](r)
}

func (a adapter) GetLatestRelease(ctx context.Context, owner, repo string) (model.Release, error) {
	r, err := a.rel.GetLatestRelease(ctx, owner, repo)
	if err != nil {
		return model.Release{}, err
	}
	return convert[model.Release](r)
}

func (a adapter) CreateRelease(ctx context.Context, owner, repo string, in apprelease.CreateInput) (model.Release, error) {
	r, err := a.rel.CreateRelease(ctx, owner, repo, forgejo_sdk.CreateReleaseOption{
		TagName: in.TagName, Target: in.Target, Title: in.Title, Note: in.Note,
		IsDraft: in.IsDraft, IsPrerelease: in.IsPrerelease,
	})
	if err != nil {
		return model.Release{}, err
	}
	return convert[model.Release](r)
}

func (a adapter) EditRelease(ctx context.Context, owner, repo string, id int64, in apprelease.EditInput) (model.Release, error) {
	r, err := a.rel.EditRelease(ctx, owner, repo, id, forgejo_sdk.EditReleaseOption{
		TagName: in.TagName, Target: in.Target, Title: in.Title, Note: in.Note,
		IsDraft: in.IsDraft, IsPrerelease: in.IsPrerelease,
	})
	if err != nil {
		return model.Release{}, err
	}
	return convert[model.Release](r)
}

func (a adapter) DeleteRelease(ctx context.Context, owner, repo string, id int64) error {
	return a.rel.DeleteRelease(ctx, owner, repo, id)
}

func (a adapter) DeleteReleaseByTag(ctx context.Context, owner, repo, tag string) error {
	return a.rel.DeleteReleaseByTag(ctx, owner, repo, tag)
}

func (a adapter) ListAttachments(ctx context.Context, owner, repo string, releaseID int64) ([]*model.Attachment, error) {
	all, err := a.rel.ListReleaseAttachments(ctx, owner, repo, releaseID)
	if err != nil {
		return nil, err
	}
	return convert[[]*model.Attachment](all)
}

func (a adapter) GetAttachment(ctx context.Context, owner, repo string, releaseID, id int64) (model.Attachment, error) {
	x, err := a.rel.GetReleaseAttachment(ctx, owner, repo, releaseID, id)
	if err != nil {
		return model.Attachment{}, err
	}
	return convert[model.Attachment](x)
}

func (a adapter) CreateAttachment(ctx context.Context, owner, repo string, releaseID int64, up apprelease.Upload) (model.Attachment, error) {
	x, err := a.rel.CreateReleaseAttachment(ctx, owner, repo, releaseID, forge.Upload{Filename: up.Filename, MIMEType: up.MIMEType, Content: up.Content})
	if err != nil {
		return model.Attachment{}, err
	}
	return convert[model.Attachment](x)
}

func (a adapter) EditAttachment(ctx context.Context, owner, repo string, releaseID, id int64, name string) (model.Attachment, error) {
	x, err := a.rel.EditReleaseAttachment(ctx, owner, repo, releaseID, id, name)
	if err != nil {
		return model.Attachment{}, err
	}
	return convert[model.Attachment](x)
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
