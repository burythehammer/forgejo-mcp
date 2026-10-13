// SPDX-License-Identifier: GPL-3.0-or-later

// Package user holds the user and notification use cases behind the user
// tools. Like all of app/ it is MCP-free and SDK-free
// (scripts/ci/check-app-boundary.sh): use cases take typed inputs, reach
// Forgejo only through Port, and return repo-owned types from app/model for an
// MCP adapter in operation/user to render.
package user

import (
	"context"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
)

// Port is the user domain as the use cases need it, in repo-owned types.
// pkg/forge/useradapter implements it over forge.Users.
type Port interface {
	// MaxResponseItems is the instance's pagination ceiling; ok is false when it
	// cannot be determined, which is "unknown", never a ceiling of zero.
	MaxResponseItems(ctx context.Context) (limit int, ok bool)
	GetMyUserInfo(ctx context.Context) (*model.User, error)
	ListNotifications(ctx context.Context, q NotificationQuery) ([]*model.NotificationThread, error)
	ListRepoNotifications(ctx context.Context, owner, repo string, q NotificationQuery) ([]*model.NotificationThread, error)
	GetNotification(ctx context.Context, id int64) (*model.NotificationThread, error)
	// ReadNotification marks one thread read and returns it.
	ReadNotification(ctx context.Context, id int64) (*model.NotificationThread, error)
	ReadNotifications(ctx context.Context, in ReadQuery) error
	ReadRepoNotifications(ctx context.Context, owner, repo string, in ReadQuery) error
}

// Source yields the port for one request. The use cases call it only after
// they have checked their input, so a request that is invalid is refused the
// same way whether or not the request's credential resolves.
type Source func(ctx context.Context) (Port, error)

// NotificationQuery filters and pages a notification list. UnreadOnly limits it
// to unread threads; zero Since and Before mean no bound.
type NotificationQuery struct {
	UnreadOnly    bool
	Since, Before time.Time
	Page, Limit   int
}

// ReadQuery marks notifications read up to LastReadAt; zero means now.
type ReadQuery struct{ LastReadAt time.Time }
