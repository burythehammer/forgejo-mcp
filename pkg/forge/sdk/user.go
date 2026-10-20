// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var _ forge.Users = (*Adapter)(nil)

func (a *Adapter) GetMyUserInfo(_ context.Context) (*forgejo_sdk.User, error) {
	u, resp, err := a.client.GetMyUserInfo()
	return u, statusErr(resp, err)
}

func (a *Adapter) GetUserInfo(_ context.Context, user string) (*forgejo_sdk.User, error) {
	u, resp, err := a.client.GetUserInfo(user)
	return u, statusErr(resp, err)
}

func (a *Adapter) ListNotifications(_ context.Context, opt forgejo_sdk.ListNotificationOptions) ([]*forgejo_sdk.NotificationThread, error) {
	n, resp, err := a.client.ListNotifications(opt)
	return n, statusErr(resp, err)
}

func (a *Adapter) ListRepoNotifications(_ context.Context, owner, repo string, opt forgejo_sdk.ListNotificationOptions) ([]*forgejo_sdk.NotificationThread, error) {
	n, resp, err := a.client.ListRepoNotifications(owner, repo, opt)
	return n, statusErr(resp, err)
}

func (a *Adapter) GetNotification(_ context.Context, id int64) (*forgejo_sdk.NotificationThread, error) {
	n, resp, err := a.client.GetNotification(id)
	return n, statusErr(resp, err)
}

func (a *Adapter) ReadNotification(_ context.Context, id int64) (*forgejo_sdk.NotificationThread, error) {
	n, resp, err := a.client.ReadNotification(id)
	return n, statusErr(resp, err)
}

func (a *Adapter) ReadNotifications(_ context.Context, opt forgejo_sdk.MarkNotificationOptions) error {
	_, resp, err := a.client.ReadNotifications(opt)
	return statusErr(resp, err)
}

func (a *Adapter) ReadRepoNotifications(_ context.Context, owner, repo string, opt forgejo_sdk.MarkNotificationOptions) error {
	_, resp, err := a.client.ReadRepoNotifications(owner, repo, opt)
	return statusErr(resp, err)
}
