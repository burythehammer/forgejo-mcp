// SPDX-License-Identifier: GPL-3.0-or-later

// Package useradapter implements the user use cases' port (app/user.Port) over
// the Forgejo port (forge.Users). It is the one place that translates between
// repo-owned types and SDK types for the user and notification domain: inputs
// become SDK options, SDK results become app/model values. Results are
// converted through their JSON form, which is exactly what a tool renders, so a
// field the repo-owned type shares by JSON name cannot drift in value.
package useradapter

import (
	"context"
	"encoding/json"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
	appuser "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/user"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// New returns the use cases' port over u.
func New(u forge.Users) appuser.Port { return adapter{u: u} }

type adapter struct{ u forge.Users }

var _ appuser.Port = adapter{}

// convert maps an SDK value onto its repo-owned twin through JSON.
func convert[T any](from any, err error) (T, error) {
	var to T
	if err != nil {
		return to, err
	}
	b, err := json.Marshal(from)
	if err != nil {
		return to, err
	}
	err = json.Unmarshal(b, &to)
	return to, err
}

func listOptions(q appuser.NotificationQuery) forgejo_sdk.ListNotificationOptions {
	opt := forgejo_sdk.ListNotificationOptions{
		ListOptions: forgejo_sdk.ListOptions{Page: q.Page, PageSize: q.Limit},
		Since:       q.Since,
		Before:      q.Before,
	}
	if q.UnreadOnly {
		opt.Status = []forgejo_sdk.NotifyStatus{forgejo_sdk.NotifyStatusUnread}
	}
	return opt
}

func markOptions(in appuser.ReadQuery) forgejo_sdk.MarkNotificationOptions {
	return forgejo_sdk.MarkNotificationOptions{LastReadAt: in.LastReadAt}
}

func (a adapter) GetMyUserInfo(ctx context.Context) (*model.User, error) {
	u, err := a.u.GetMyUserInfo(ctx)
	return convert[*model.User](u, err)
}

func (a adapter) ListNotifications(ctx context.Context, q appuser.NotificationQuery) ([]*model.NotificationThread, error) {
	threads, err := a.u.ListNotifications(ctx, listOptions(q))
	return convert[[]*model.NotificationThread](threads, err)
}

func (a adapter) ListRepoNotifications(ctx context.Context, owner, repo string, q appuser.NotificationQuery) ([]*model.NotificationThread, error) {
	threads, err := a.u.ListRepoNotifications(ctx, owner, repo, listOptions(q))
	return convert[[]*model.NotificationThread](threads, err)
}

func (a adapter) GetNotification(ctx context.Context, id int64) (*model.NotificationThread, error) {
	t, err := a.u.GetNotification(ctx, id)
	return convert[*model.NotificationThread](t, err)
}

func (a adapter) ReadNotification(ctx context.Context, id int64) (*model.NotificationThread, error) {
	t, err := a.u.ReadNotification(ctx, id)
	return convert[*model.NotificationThread](t, err)
}

func (a adapter) ReadNotifications(ctx context.Context, in appuser.ReadQuery) error {
	return a.u.ReadNotifications(ctx, markOptions(in))
}

func (a adapter) ReadRepoNotifications(ctx context.Context, owner, repo string, in appuser.ReadQuery) error {
	return a.u.ReadRepoNotifications(ctx, owner, repo, markOptions(in))
}
