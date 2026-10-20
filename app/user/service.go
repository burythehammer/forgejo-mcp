// SPDX-License-Identifier: GPL-3.0-or-later

package user

import (
	"context"
	"fmt"
	"time"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/list"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/model"
)

// Service runs the user and notification use cases. It holds only its Source;
// the port is looked up per call, after the input has been checked.
type Service struct{ src Source }

// New returns the use cases over src.
func New(src Source) *Service { return &Service{src: src} }

// NotificationsInput lists notifications. All includes read threads; Since and
// Before are RFC 3339 text, or empty for no bound.
type NotificationsInput struct {
	All           bool
	Since, Before string
	Page, Limit   int
}

// RepoNotificationsInput lists the notifications of one repository.
type RepoNotificationsInput struct {
	Owner, Repo string
	NotificationsInput
}

// MyInfo returns the user the credential belongs to.
func (s *Service) MyInfo(ctx context.Context) (*model.User, error) {
	p, err := s.port(ctx)
	if err != nil {
		return nil, err
	}
	u, err := p.GetMyUserInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("get user info err: %w", err)
	}
	return u, nil
}

// Notifications returns one page of the credential's user's notifications,
// with its next marker.
func (s *Service) Notifications(ctx context.Context, in NotificationsInput) (model.NotificationPage, error) {
	q, err := notificationQuery(in)
	if err != nil {
		return model.NotificationPage{}, err
	}
	p, err := s.port(ctx)
	if err != nil {
		return model.NotificationPage{}, err
	}
	ceiling, known := p.MaxResponseItems(ctx)
	c := list.Ceiling{Max: ceiling, Known: known}
	page, err := list.Run(ctx, list.Normalize(list.Paging{Page: q.Page, Limit: q.Limit}, list.DefaultLimit(20, c)), c,
		func(ctx context.Context, lp list.Paging) (list.Batch[*model.NotificationThread], error) {
			qq := q
			qq.Page, qq.Limit = lp.Page, lp.Limit
			threads, err := p.ListNotifications(ctx, qq)
			if err != nil {
				return list.Batch[*model.NotificationThread]{}, fmt.Errorf("list notifications err: %w", err)
			}
			return list.Batch[*model.NotificationThread]{Items: threads}, nil
		})
	if err != nil {
		return model.NotificationPage{}, err
	}
	return model.NotificationPage{
		Notifications: page.Items, PageMeta: model.PageMetaOf(page),
	}, nil
}

// RepoNotifications returns one page of the notifications of one repository,
// with its next marker.
func (s *Service) RepoNotifications(ctx context.Context, in RepoNotificationsInput) (model.NotificationPage, error) {
	q, err := notificationQuery(in.NotificationsInput)
	if err != nil {
		return model.NotificationPage{}, err
	}
	p, err := s.port(ctx)
	if err != nil {
		return model.NotificationPage{}, err
	}
	ceiling, known := p.MaxResponseItems(ctx)
	c := list.Ceiling{Max: ceiling, Known: known}
	page, err := list.Run(ctx, list.Normalize(list.Paging{Page: q.Page, Limit: q.Limit}, list.DefaultLimit(20, c)), c,
		func(ctx context.Context, lp list.Paging) (list.Batch[*model.NotificationThread], error) {
			qq := q
			qq.Page, qq.Limit = lp.Page, lp.Limit
			threads, err := p.ListRepoNotifications(ctx, in.Owner, in.Repo, qq)
			if err != nil {
				return list.Batch[*model.NotificationThread]{}, fmt.Errorf("list repo notifications err: %w", err)
			}
			return list.Batch[*model.NotificationThread]{Items: threads}, nil
		})
	if err != nil {
		return model.NotificationPage{}, err
	}
	return model.NotificationPage{
		Notifications: page.Items, PageMeta: model.PageMetaOf(page),
	}, nil
}

// Thread returns one notification thread.
func (s *Service) Thread(ctx context.Context, id int64) (*model.NotificationThread, error) {
	p, err := s.port(ctx)
	if err != nil {
		return nil, err
	}
	thread, err := p.GetNotification(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get notification thread err: %w", err)
	}
	return thread, nil
}

// MarkRead marks one notification thread read and returns it.
func (s *Service) MarkRead(ctx context.Context, id int64) (*model.NotificationThread, error) {
	p, err := s.port(ctx)
	if err != nil {
		return nil, err
	}
	thread, err := p.ReadNotification(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("mark notification read err: %w", err)
	}
	return thread, nil
}

// MarkAllRead marks every notification read up to lastReadAt (RFC 3339, or
// empty for now).
func (s *Service) MarkAllRead(ctx context.Context, lastReadAt string) error {
	in, err := readQuery(lastReadAt)
	if err != nil {
		return err
	}
	p, err := s.port(ctx)
	if err != nil {
		return err
	}
	if err := p.ReadNotifications(ctx, in); err != nil {
		return fmt.Errorf("mark all notifications read err: %w", err)
	}
	return nil
}

// MarkRepoRead marks every notification of one repository read up to lastReadAt
// (RFC 3339, or empty for now).
func (s *Service) MarkRepoRead(ctx context.Context, owner, repo, lastReadAt string) error {
	in, err := readQuery(lastReadAt)
	if err != nil {
		return err
	}
	p, err := s.port(ctx)
	if err != nil {
		return err
	}
	if err := p.ReadRepoNotifications(ctx, owner, repo, in); err != nil {
		return fmt.Errorf("mark repo notifications read err: %w", err)
	}
	return nil
}

func (s *Service) port(ctx context.Context) (Port, error) { return s.src(ctx) }

func notificationQuery(in NotificationsInput) (NotificationQuery, error) {
	q := NotificationQuery{UnreadOnly: !in.All, Page: in.Page, Limit: in.Limit}
	var err error
	if in.Since != "" {
		if q.Since, err = time.Parse(time.RFC3339, in.Since); err != nil {
			return q, &InvalidInputError{What: "invalid since time format (expected RFC3339)", Err: err}
		}
	}
	if in.Before != "" {
		if q.Before, err = time.Parse(time.RFC3339, in.Before); err != nil {
			return q, &InvalidInputError{What: "invalid before time format (expected RFC3339)", Err: err}
		}
	}
	return q, nil
}

func readQuery(lastReadAt string) (ReadQuery, error) {
	var in ReadQuery
	if lastReadAt != "" {
		t, err := time.Parse(time.RFC3339, lastReadAt)
		if err != nil {
			return in, &InvalidInputError{What: "invalid last_read_at time format (expected RFC3339)", Err: err}
		}
		in.LastReadAt = t
	}
	return in, nil
}
