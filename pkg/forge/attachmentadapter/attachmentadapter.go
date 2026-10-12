// SPDX-License-Identifier: GPL-3.0-or-later

// Package attachmentadapter implements the attachment use cases' port
// (app/attachment.Port) over the Forgejo port (forge.Attachments). It is the one
// place that translates between repo-owned types and SDK types for issue and
// comment attachments.
package attachmentadapter

import (
	"context"
	"errors"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/attachment"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/download"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/bridge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// New returns the use cases' port over p.
func New(p forge.Attachments) attachment.Port { return adapter{p: p} }

type adapter struct{ p forge.Attachments }

var _ attachment.Port = adapter{}

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
	if forge.IsNotFound(err) {
		return nil, attachment.ErrNotFound
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
// re-expressed as the use case's, so the use case never imports pkg/forge.
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
