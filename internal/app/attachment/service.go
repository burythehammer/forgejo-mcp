// SPDX-License-Identifier: GPL-3.0-or-later

package attachment

import (
	"context"
	"errors"
	"fmt"
	"time"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/download"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/list"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/ptr"
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

// Paging is one page request: a 1-based page and an item limit.
type Paging struct{ Page, Limit int }

// defaultLimit is the page size an attachment list uses when the caller gives none.
const defaultLimit = 20

// List returns one page of the attachments of a parent with its next marker.
// Forgejo does not page this listing, so the whole list is read and cut here.
// A parent Forgejo does not know is an empty list. The page's attachments are
// never nil.
func (s *Service) List(ctx context.Context, owner, repo string, sc Scope, parent int64, p Paging) (list.Bounded[*model.Attachment], error) {
	max, known := s.p.MaxResponseItems(ctx)
	c := list.Ceiling{Max: max, Known: known}
	page, err := list.Run(ctx, list.Given(list.Paging(p), defaultLimit, c), c,
		func(ctx context.Context, q list.Paging) (list.Batch[*model.Attachment], error) {
			all, err := s.p.List(ctx, owner, repo, sc, parent)
			if err != nil && !errors.Is(err, ErrNotFound) {
				return list.Batch[*model.Attachment]{}, fmt.Errorf("list %s attachments err: %w", sc.label(), err)
			}
			rows := list.Cut(all, q).Items
			return list.Batch[*model.Attachment]{Items: rows, Total: ptr.To(len(all))}, nil
		})
	if err != nil {
		return list.Bounded[*model.Attachment]{}, err
	}
	if page.Items == nil {
		page.Items = []*model.Attachment{}
	}
	return page, nil
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
