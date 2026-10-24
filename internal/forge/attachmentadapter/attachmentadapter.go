// SPDX-License-Identifier: GPL-3.0-or-later

// Package attachmentadapter implements the attachment use cases' port
// (internal/app/attachment.Port) over the Forgejo port (forge.Attachments). It is the one
// place that translates between repo-owned types and SDK types for issue and
// comment attachments.
package attachmentadapter

import (
	"context"
	"errors"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/attachment"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/download"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/bridge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// AttachmentSource is the part of the Forgejo port the attachment domain reads
// through: its attachment methods and the instance's pagination ceiling.
type AttachmentSource interface {
	forge.Attachments
	forge.Ceilings
}

// New returns the use cases' port over p.
func New(p AttachmentSource) attachment.Port { return adapter{p: p} }

type adapter struct{ p AttachmentSource }

var _ attachment.Port = adapter{}

// MaxResponseItems reads the instance's pagination ceiling through the source.
func (a adapter) MaxResponseItems(ctx context.Context) (int, bool) {
	return a.p.MaxResponseItems(ctx)
}

func (a adapter) List(ctx context.Context, owner, repo string, s attachment.Scope, parent int64) ([]*model.Attachment, error) {
	var (
		out []*forgejo_sdk.Attachment
		err error
	)
	if s == attachment.ScopeComment {
		out, err = a.p.ListCommentAttachments(ctx, owner, repo, parent)
	} else {
		out, err = a.p.ListIssueAttachments(ctx, owner, repo, parent)
	}
	return bridge.Convert[[]*model.Attachment](out, err)
}

func (a adapter) Get(ctx context.Context, owner, repo string, s attachment.Scope, parent, id int64) (model.Attachment, error) {
	var (
		x   *forgejo_sdk.Attachment
		err error
	)
	if s == attachment.ScopeComment {
		x, err = a.p.GetCommentAttachment(ctx, owner, repo, parent, id)
	} else {
		x, err = a.p.GetIssueAttachment(ctx, owner, repo, parent, id)
	}
	return bridge.Convert[model.Attachment](x, err)
}

func (a adapter) Create(ctx context.Context, owner, repo string, s attachment.Scope, parent int64, up attachment.Upload) (model.Attachment, error) {
	f := forge.Upload{Filename: up.Filename, MIMEType: up.MIMEType, Content: up.Content}
	var (
		x   *forgejo_sdk.Attachment
		err error
	)
	if s == attachment.ScopeComment {
		x, err = a.p.CreateCommentAttachment(ctx, owner, repo, parent, f)
	} else {
		x, err = a.p.CreateIssueAttachment(ctx, owner, repo, parent, f)
	}
	return bridge.Convert[model.Attachment](x, err)
}

func (a adapter) Edit(ctx context.Context, owner, repo string, s attachment.Scope, parent, id int64, name string) (model.Attachment, error) {
	var (
		x   *forgejo_sdk.Attachment
		err error
	)
	if s == attachment.ScopeComment {
		x, err = a.p.EditCommentAttachment(ctx, owner, repo, parent, id, name)
	} else {
		x, err = a.p.EditIssueAttachment(ctx, owner, repo, parent, id, name)
	}
	return bridge.Convert[model.Attachment](x, err)
}

func (a adapter) Delete(ctx context.Context, owner, repo string, s attachment.Scope, parent, id int64) error {
	if s == attachment.ScopeComment {
		return a.p.DeleteCommentAttachment(ctx, owner, repo, parent, id)
	}
	return a.p.DeleteIssueAttachment(ctx, owner, repo, parent, id)
}

// Download fetches an attachment's bytes. The forge's inline-cap sentinel is
// re-expressed as the use case's, so the use case never imports internal/forge.
func (a adapter) Download(ctx context.Context, url string) (model.AttachmentBody, error) {
	d, err := a.p.DownloadAttachment(ctx, url)
	if errors.Is(err, forge.ErrPayloadTooLarge) {
		return model.AttachmentBody{}, download.ErrPayloadTooLarge
	}
	if err != nil {
		return model.AttachmentBody{}, err
	}
	return model.AttachmentBody{Body: d.Body, ContentType: d.ContentType}, nil
}
