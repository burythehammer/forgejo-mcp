// SPDX-License-Identifier: GPL-3.0-or-later

package attachment

import (
	"context"
	"errors"
	"fmt"
	"time"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/download"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
)

// Service is the attachment use cases: one method per scope and action.
type Service struct {
	p Port
	// uploadTimeout bounds one create call, so a stuck network path fails with a
	// clear error instead of hanging. Zero means no bound of its own.
	uploadTimeout time.Duration
}

// New returns the use cases over p. uploadTimeout bounds each create call.
func New(p Port, uploadTimeout time.Duration) *Service {
	return &Service{p: p, uploadTimeout: uploadTimeout}
}

// List returns the attachments of a parent. A parent Forgejo does not know is
// an empty list. The result is never nil.
func (s *Service) List(ctx context.Context, owner, repo string, sc Scope, parent int64) ([]*model.Attachment, error) {
	out, err := s.p.List(ctx, owner, repo, sc, parent)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, fmt.Errorf("list %s attachments err: %w", sc.label(), err)
	}
	if out == nil {
		out = []*model.Attachment{}
	}
	return out, nil
}

// Get returns one attachment's metadata.
func (s *Service) Get(ctx context.Context, owner, repo string, sc Scope, parent, id int64) (model.Attachment, error) {
	a, err := s.p.Get(ctx, owner, repo, sc, parent, id)
	if err != nil {
		return model.Attachment{}, fmt.Errorf("get %s attachment err: %w", sc.label(), err)
	}
	return a, nil
}

// Create uploads an attachment, bounded by the upload timeout.
func (s *Service) Create(ctx context.Context, owner, repo string, sc Scope, parent int64, up Upload) (model.Attachment, error) {
	uploadCtx := ctx
	if s.uploadTimeout > 0 {
		var cancel context.CancelFunc
		uploadCtx, cancel = context.WithTimeout(ctx, s.uploadTimeout)
		defer cancel()
	}
	a, err := s.p.Create(uploadCtx, owner, repo, sc, parent, up)
	if err != nil {
		return model.Attachment{}, fmt.Errorf("create %s attachment err: %w", sc.label(), s.wrapUploadTimeout(ctx, uploadCtx, err))
	}
	return a, nil
}

// wrapUploadTimeout names the upload's own budget when that is what ran out. A
// deadline the caller set is not misattributed.
func (s *Service) wrapUploadTimeout(parent, uploadCtx context.Context, err error) error {
	if !errors.Is(uploadCtx.Err(), context.DeadlineExceeded) {
		return err
	}
	if errors.Is(parent.Err(), context.DeadlineExceeded) {
		return err
	}
	return fmt.Errorf("upload timed out after %s: %w", s.uploadTimeout, err)
}

// Edit renames an attachment.
func (s *Service) Edit(ctx context.Context, owner, repo string, sc Scope, parent, id int64, name string) (model.Attachment, error) {
	a, err := s.p.Edit(ctx, owner, repo, sc, parent, id, name)
	if err != nil {
		return model.Attachment{}, fmt.Errorf("edit %s attachment err: %w", sc.label(), err)
	}
	return a, nil
}

// Delete removes an attachment.
func (s *Service) Delete(ctx context.Context, owner, repo string, sc Scope, parent, id int64) error {
	if err := s.p.Delete(ctx, owner, repo, sc, parent, id); err != nil {
		return fmt.Errorf("delete %s attachment err: %w", sc.label(), err)
	}
	return nil
}

// Download is the outcome of a download tool: the result to render and, when
// the bytes are inline, the body to ship with it.
type Download struct {
	Result model.DownloadResult
	Body   model.AttachmentBody
}

// Download reads an attachment's metadata and, when it is under the inline cap,
// its bytes.
func (s *Service) Download(ctx context.Context, owner, repo string, sc Scope, parent, id int64) (Download, error) {
	a, err := s.p.Get(ctx, owner, repo, sc, parent, id)
	if err != nil {
		return Download{}, fmt.Errorf("download %s attachment (metadata) err: %w", sc.label(), err)
	}
	res, body, err := download.Resolve(ctx, &a, s.p.Download)
	if err != nil {
		return Download{}, fmt.Errorf("download body err: %w", err)
	}
	return Download{Result: res, Body: body}, nil
}
