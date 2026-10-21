// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"net/http"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var _ forge.Releases = (*Adapter)(nil)

func (a *Adapter) ListReleases(_ context.Context, owner, repo string, opt forgejo_sdk.ListReleasesOptions) ([]*forgejo_sdk.Release, error) {
	rels, resp, err := a.client.ListReleases(owner, repo, opt)
	return rels, statusErr(resp, err)
}

func (a *Adapter) GetRelease(_ context.Context, owner, repo string, id int64) (*forgejo_sdk.Release, error) {
	rel, resp, err := a.client.GetRelease(owner, repo, id)
	return rel, statusErr(resp, err)
}

func (a *Adapter) GetReleaseByTag(_ context.Context, owner, repo, tag string) (*forgejo_sdk.Release, error) {
	rel, resp, err := a.client.GetReleaseByTag(owner, repo, tag)
	return rel, statusErr(resp, err)
}

func (a *Adapter) GetLatestRelease(_ context.Context, owner, repo string) (*forgejo_sdk.Release, error) {
	rel, resp, err := a.client.GetLatestRelease(owner, repo)
	return rel, statusErr(resp, err)
}

func (a *Adapter) CreateRelease(_ context.Context, owner, repo string, opt forgejo_sdk.CreateReleaseOption) (*forgejo_sdk.Release, error) {
	rel, resp, err := a.client.CreateRelease(owner, repo, opt)
	return rel, statusErr(resp, err)
}

func (a *Adapter) EditRelease(_ context.Context, owner, repo string, id int64, opt forgejo_sdk.EditReleaseOption) (*forgejo_sdk.Release, error) {
	rel, resp, err := a.client.EditRelease(owner, repo, id, opt)
	return rel, statusErr(resp, err)
}

func (a *Adapter) DeleteRelease(_ context.Context, owner, repo string, id int64) error {
	resp, err := a.client.DeleteRelease(owner, repo, id)
	return statusErr(resp, err)
}

func (a *Adapter) DeleteReleaseByTag(_ context.Context, owner, repo, tag string) error {
	resp, err := a.client.DeleteReleaseByTag(owner, repo, tag)
	return statusErr(resp, err)
}

func (a *Adapter) ListReleaseAttachments(_ context.Context, owner, repo string, releaseID int64) ([]*forgejo_sdk.Attachment, error) {
	atts, resp, err := a.client.ListReleaseAttachments(owner, repo, releaseID)
	return atts, statusErr(resp, err)
}

func (a *Adapter) GetReleaseAttachment(_ context.Context, owner, repo string, releaseID, id int64) (*forgejo_sdk.Attachment, error) {
	att, resp, err := a.client.GetReleaseAttachment(owner, repo, releaseID, id)
	return att, statusErr(resp, err)
}

// CreateReleaseAttachment streams the upload over raw multipart HTTP: the
// SDK's equivalent buffers the file and cannot set a content type.
func (a *Adapter) CreateReleaseAttachment(ctx context.Context, owner, repo string, releaseID int64, up forge.Upload) (*forgejo_sdk.Attachment, error) {
	return a.uploadAttachment(ctx, forge.APIPath("repos", owner, repo, "releases", releaseID, "assets"), up)
}

func (a *Adapter) EditReleaseAttachment(_ context.Context, owner, repo string, releaseID, id int64, name string) (*forgejo_sdk.Attachment, error) {
	att, resp, err := a.client.EditReleaseAttachment(owner, repo, releaseID, id, forgejo_sdk.EditAttachmentOptions{Name: name})
	return att, statusErr(resp, err)
}

func (a *Adapter) DeleteReleaseAttachment(_ context.Context, owner, repo string, releaseID, id int64) error {
	resp, err := a.client.DeleteReleaseAttachment(owner, repo, releaseID, id)
	return statusErr(resp, err)
}

// uploadAttachment POSTs up as multipart form data to path and decodes the
// attachment Forgejo answers with.
func (a *Adapter) uploadAttachment(ctx context.Context, path string, up forge.Upload) (*forgejo_sdk.Attachment, error) {
	var att forgejo_sdk.Attachment
	if err := a.raw.doMultipart(ctx, http.MethodPost, path, "attachment", up.Filename, up.MIMEType, up.Content, &att); err != nil {
		return nil, err
	}
	return &att, nil
}
