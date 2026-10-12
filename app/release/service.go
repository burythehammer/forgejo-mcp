// SPDX-License-Identifier: GPL-3.0-or-later

package release

import (
	"context"
	"fmt"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/download"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/list"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
)

// Service is the release use cases: one method per tool.
type Service struct{ p Port }

// New returns the use cases over p.
func New(p Port) *Service { return &Service{p: p} }

// ListQuery filters and pages a release list. State is one of the State
// constants; the empty string means all.
type ListQuery struct {
	State string
	Paging
}

// ListReleases returns one page of releases, filtered by state after Forgejo
// has paged them. The result is never nil.
func (s *Service) ListReleases(ctx context.Context, owner, repo string, q ListQuery) ([]*model.Release, error) {
	ceiling, known := s.p.MaxResponseItems(ctx)
	c := list.Ceiling{Max: ceiling, Known: known}
	rels, err := list.Items(ctx, list.Normalize(list.Paging(q.Paging), list.DefaultLimit(defaultLimit, c)), c,
		func(ctx context.Context, p list.Paging) (list.Batch[*model.Release], error) {
			got, err := s.p.ListReleases(ctx, owner, repo, Paging(p))
			if err != nil {
				return list.Batch[*model.Release]{}, fmt.Errorf("list releases err: %w", err)
			}
			// The state filter applies to the page Forgejo returned, as it always has.
			return list.Batch[*model.Release]{Items: filterByState(got, q.State)}, nil
		})
	if err != nil {
		return nil, err
	}
	if rels == nil {
		rels = []*model.Release{}
	}
	return rels, nil
}

// defaultLimit is the page size a release list uses when the caller gives none.
const defaultLimit = 20

func filterByState(rels []*model.Release, state string) []*model.Release {
	if state == "" || state == StateAll {
		return rels
	}
	out := make([]*model.Release, 0, len(rels))
	for _, r := range rels {
		if r == nil {
			continue
		}
		switch state {
		case StateDraft:
			if r.IsDraft {
				out = append(out, r)
			}
		case StatePrerelease:
			if !r.IsDraft && r.IsPrerelease {
				out = append(out, r)
			}
		case StatePublished:
			if !r.IsDraft && !r.IsPrerelease {
				out = append(out, r)
			}
		}
	}
	return out
}

// GetRelease returns one release by ID.
func (s *Service) GetRelease(ctx context.Context, owner, repo string, id int64) (model.Release, error) {
	rel, err := s.p.GetRelease(ctx, owner, repo, id)
	if err != nil {
		return model.Release{}, fmt.Errorf("get release err: %w", err)
	}
	return rel, nil
}

// GetReleaseByTag returns the release for a tag.
func (s *Service) GetReleaseByTag(ctx context.Context, owner, repo, tag string) (model.Release, error) {
	rel, err := s.p.GetReleaseByTag(ctx, owner, repo, tag)
	if err != nil {
		return model.Release{}, fmt.Errorf("get release by tag err: %w", err)
	}
	return rel, nil
}

// GetLatestRelease returns the newest release that is neither a draft nor a
// prerelease.
func (s *Service) GetLatestRelease(ctx context.Context, owner, repo string) (model.Release, error) {
	rel, err := s.p.GetLatestRelease(ctx, owner, repo)
	if err != nil {
		return model.Release{}, fmt.Errorf("get latest release err: %w", err)
	}
	return rel, nil
}

// CreateRelease creates a release. An empty Title defaults to the tag name,
// because Forgejo rejects an empty one.
func (s *Service) CreateRelease(ctx context.Context, owner, repo string, in CreateInput) (model.Release, error) {
	if in.Title == "" {
		in.Title = in.TagName
	}
	rel, err := s.p.CreateRelease(ctx, owner, repo, in)
	if err != nil {
		return model.Release{}, fmt.Errorf("create release err: %w", err)
	}
	return rel, nil
}

// EditRelease changes the fields in supplies.
func (s *Service) EditRelease(ctx context.Context, owner, repo string, id int64, in EditInput) (model.Release, error) {
	rel, err := s.p.EditRelease(ctx, owner, repo, id, in)
	if err != nil {
		return model.Release{}, fmt.Errorf("edit release err: %w", err)
	}
	return rel, nil
}

// DeleteRelease deletes a release by ID.
func (s *Service) DeleteRelease(ctx context.Context, owner, repo string, id int64) error {
	if err := s.p.DeleteRelease(ctx, owner, repo, id); err != nil {
		return fmt.Errorf("delete release err: %w", err)
	}
	return nil
}

// DeleteReleaseByTag deletes the release for a tag.
func (s *Service) DeleteReleaseByTag(ctx context.Context, owner, repo, tag string) error {
	if err := s.p.DeleteReleaseByTag(ctx, owner, repo, tag); err != nil {
		return fmt.Errorf("delete release by tag err: %w", err)
	}
	return nil
}

// ListAttachments returns one page of a release's assets. Forgejo does not
// page this endpoint, so the page is cut here.
func (s *Service) ListAttachments(ctx context.Context, owner, repo string, releaseID int64, p Paging) ([]*model.Attachment, error) {
	ceiling, known := s.p.MaxResponseItems(ctx)
	c := list.Ceiling{Max: ceiling, Known: known}
	out, err := list.Items(ctx, list.Normalize(list.Paging(p), list.DefaultLimit(defaultLimit, c)), c,
		func(ctx context.Context, q list.Paging) (list.Batch[*model.Attachment], error) {
			all, err := s.p.ListAttachments(ctx, owner, repo, releaseID)
			if err != nil {
				return list.Batch[*model.Attachment]{}, fmt.Errorf("list release attachments err: %w", err)
			}
			// Forgejo does not page the attachment list, so the window is cut here.
			return list.Batch[*model.Attachment]{Items: slice(all, q.Page, q.Limit)}, nil
		})
	if err != nil {
		return nil, err
	}
	if out == nil {
		out = []*model.Attachment{}
	}
	return out, nil
}

// slice is the window of page and limit over all, empty past its end.
func slice(all []*model.Attachment, page, limit int) []*model.Attachment {
	offset := (page - 1) * limit
	if offset >= len(all) {
		return []*model.Attachment{}
	}
	end := min(offset+limit, len(all))
	return all[offset:end]
}

// GetAttachment returns one asset's metadata.
func (s *Service) GetAttachment(ctx context.Context, owner, repo string, releaseID, id int64) (model.Attachment, error) {
	a, err := s.p.GetAttachment(ctx, owner, repo, releaseID, id)
	if err != nil {
		return model.Attachment{}, fmt.Errorf("get release attachment err: %w", err)
	}
	return a, nil
}

// Download is the outcome of download_release_attachment: the result to render
// and, when the bytes are inline, the body to ship with it.
type Download struct {
	Result model.DownloadResult
	Body   model.AttachmentBody
}

// DownloadAttachment reads an asset's metadata and, when it is under the
// inline cap, its bytes.
func (s *Service) DownloadAttachment(ctx context.Context, owner, repo string, releaseID, id int64) (Download, error) {
	att, err := s.p.GetAttachment(ctx, owner, repo, releaseID, id)
	if err != nil {
		return Download{}, fmt.Errorf("download release attachment (metadata) err: %w", err)
	}
	res, body, err := download.Resolve(ctx, &att, s.p.DownloadAttachment)
	if err != nil {
		return Download{}, fmt.Errorf("download body err: %w", err)
	}
	return Download{Result: res, Body: body}, nil
}

// CreateAttachment uploads an asset to a release.
func (s *Service) CreateAttachment(ctx context.Context, owner, repo string, releaseID int64, up Upload) (model.Attachment, error) {
	a, err := s.p.CreateAttachment(ctx, owner, repo, releaseID, up)
	if err != nil {
		return model.Attachment{}, fmt.Errorf("create release attachment err: %w", err)
	}
	return a, nil
}

// EditAttachment renames an asset.
func (s *Service) EditAttachment(ctx context.Context, owner, repo string, releaseID, id int64, name string) (model.Attachment, error) {
	a, err := s.p.EditAttachment(ctx, owner, repo, releaseID, id, name)
	if err != nil {
		return model.Attachment{}, fmt.Errorf("edit release attachment err: %w", err)
	}
	return a, nil
}

// DeleteAttachment deletes an asset.
func (s *Service) DeleteAttachment(ctx context.Context, owner, repo string, releaseID, id int64) error {
	if err := s.p.DeleteAttachment(ctx, owner, repo, releaseID, id); err != nil {
		return fmt.Errorf("delete release attachment err: %w", err)
	}
	return nil
}
