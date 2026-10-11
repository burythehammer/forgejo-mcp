// SPDX-License-Identifier: GPL-3.0-or-later

package forge

import (
	"context"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// Users is the user domain of the port: user profiles and the credential's
// own notifications.
//
// Methods are named for what the user tools and the owner resource need, and
// use SDK types as plain data. Failures from Forgejo are *StatusError.
type Users interface {
	// GetMyUserInfo returns the user the credential belongs to.
	GetMyUserInfo(ctx context.Context) (*forgejo_sdk.User, error)
	GetUserInfo(ctx context.Context, user string) (*forgejo_sdk.User, error)

	ListNotifications(ctx context.Context, opt forgejo_sdk.ListNotificationOptions) ([]*forgejo_sdk.NotificationThread, error)
	ListRepoNotifications(ctx context.Context, owner, repo string, opt forgejo_sdk.ListNotificationOptions) ([]*forgejo_sdk.NotificationThread, error)
	GetNotification(ctx context.Context, id int64) (*forgejo_sdk.NotificationThread, error)
	// ReadNotification marks one thread read and returns it.
	ReadNotification(ctx context.Context, id int64) (*forgejo_sdk.NotificationThread, error)
	// ReadNotifications marks every notification of the credential's user read.
	ReadNotifications(ctx context.Context, opt forgejo_sdk.MarkNotificationOptions) error
	// ReadRepoNotifications marks every notification in one repository read.
	ReadRepoNotifications(ctx context.Context, owner, repo string, opt forgejo_sdk.MarkNotificationOptions) error
}
