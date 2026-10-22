// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"errors"
	"net/http"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var _ forge.Attachments = (*Adapter)(nil)

// The SDK has no issue or comment attachment endpoints, so these go over raw
// HTTP.

func issueAssets(owner, repo string, index int64, rest ...any) (string, error) {
	return forge.APIPath(append([]any{"repos", owner, repo, "issues", index, "assets"}, rest...)...)
}

func commentAssets(owner, repo string, commentID int64, rest ...any) (string, error) {
	return forge.APIPath(append([]any{"repos", owner, repo, "issues", "comments", commentID, "assets"}, rest...)...)
}

func (a *Adapter) listAttachments(ctx context.Context, path string) ([]*forgejo_sdk.Attachment, error) {
	var out []*forgejo_sdk.Attachment
	if err := a.rawDo(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []*forgejo_sdk.Attachment{}
	}
	return out, nil
}

func (a *Adapter) getAttachment(ctx context.Context, path string) (*forgejo_sdk.Attachment, error) {
	var att forgejo_sdk.Attachment
	if err := a.rawDo(ctx, http.MethodGet, path, nil, &att); err != nil {
		return nil, err
	}
	return &att, nil
}

func (a *Adapter) renameAttachment(ctx context.Context, path, name string) (*forgejo_sdk.Attachment, error) {
	var att forgejo_sdk.Attachment
	if err := a.rawDo(ctx, http.MethodPatch, path, map[string]string{"name": name}, &att); err != nil {
		return nil, err
	}
	return &att, nil
}

func (a *Adapter) deleteAttachment(ctx context.Context, path string) error {
	return a.rawDo(ctx, http.MethodDelete, path, nil, nil)
}

func (a *Adapter) ListIssueAttachments(ctx context.Context, owner, repo string, index int64) ([]*forgejo_sdk.Attachment, error) {
	path, err := issueAssets(owner, repo, index)
	if err != nil {
		return nil, err
	}
	return a.listAttachments(ctx, path)
}

func (a *Adapter) GetIssueAttachment(ctx context.Context, owner, repo string, index, id int64) (*forgejo_sdk.Attachment, error) {
	path, err := issueAssets(owner, repo, index, id)
	if err != nil {
		return nil, err
	}
	return a.getAttachment(ctx, path)
}

func (a *Adapter) CreateIssueAttachment(ctx context.Context, owner, repo string, index int64, up forge.Upload) (*forgejo_sdk.Attachment, error) {
	path, err := issueAssets(owner, repo, index)
	if err != nil {
		return nil, err
	}
	return a.uploadAttachment(ctx, path, up)
}

func (a *Adapter) EditIssueAttachment(ctx context.Context, owner, repo string, index, id int64, name string) (*forgejo_sdk.Attachment, error) {
	path, err := issueAssets(owner, repo, index, id)
	if err != nil {
		return nil, err
	}
	return a.renameAttachment(ctx, path, name)
}

func (a *Adapter) DeleteIssueAttachment(ctx context.Context, owner, repo string, index, id int64) error {
	path, err := issueAssets(owner, repo, index, id)
	if err != nil {
		return err
	}
	return a.deleteAttachment(ctx, path)
}

func (a *Adapter) ListCommentAttachments(ctx context.Context, owner, repo string, commentID int64) ([]*forgejo_sdk.Attachment, error) {
	path, err := commentAssets(owner, repo, commentID)
	if err != nil {
		return nil, err
	}
	return a.listAttachments(ctx, path)
}

func (a *Adapter) GetCommentAttachment(ctx context.Context, owner, repo string, commentID, id int64) (*forgejo_sdk.Attachment, error) {
	path, err := commentAssets(owner, repo, commentID, id)
	if err != nil {
		return nil, err
	}
	return a.getAttachment(ctx, path)
}

func (a *Adapter) CreateCommentAttachment(ctx context.Context, owner, repo string, commentID int64, up forge.Upload) (*forgejo_sdk.Attachment, error) {
	path, err := commentAssets(owner, repo, commentID)
	if err != nil {
		return nil, err
	}
	return a.uploadAttachment(ctx, path, up)
}

func (a *Adapter) EditCommentAttachment(ctx context.Context, owner, repo string, commentID, id int64, name string) (*forgejo_sdk.Attachment, error) {
	path, err := commentAssets(owner, repo, commentID, id)
	if err != nil {
		return nil, err
	}
	return a.renameAttachment(ctx, path, name)
}

func (a *Adapter) DeleteCommentAttachment(ctx context.Context, owner, repo string, commentID, id int64) error {
	path, err := commentAssets(owner, repo, commentID, id)
	if err != nil {
		return err
	}
	return a.deleteAttachment(ctx, path)
}

// DownloadAttachment reads the attachment's bytes with the request's
// credential, up to the inline cap.
func (a *Adapter) DownloadAttachment(ctx context.Context, downloadURL string) (forge.Download, error) {
	body, ct, err := a.raw.doRaw(ctx, downloadURL)
	if errors.Is(err, forge.ErrPayloadTooLarge) {
		return forge.Download{}, forge.ErrPayloadTooLarge
	}
	if err != nil {
		return forge.Download{}, err
	}
	return forge.Download{Body: body, ContentType: ct}, nil
}
