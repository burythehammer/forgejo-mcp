// SPDX-License-Identifier: GPL-3.0-or-later

package fake

import (
	"context"
	"net/http"
	"sort"
	"sync"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var _ forge.Users = (*Forgejo)(nil)

type userState struct {
	mu     sync.Mutex
	users  map[string]*forgejo_sdk.User
	notifs []*forgejo_sdk.NotificationThread
	nextID int64
}

// AddUser makes the user exist. Poster, the user the fake acts as, always
// exists.
func (f *Forgejo) AddUser(login string) {
	f.users.mu.Lock()
	defer f.users.mu.Unlock()
	f.users.add(login)
}

func (s *userState) add(login string) *forgejo_sdk.User {
	if s.users == nil {
		s.users = map[string]*forgejo_sdk.User{}
	}
	u, ok := s.users[login]
	if !ok {
		u = &forgejo_sdk.User{ID: int64(len(s.users) + 1), UserName: login, FullName: login, IsActive: true}
		s.users[login] = u
	}
	return u
}

// userExists reports whether login is a known user. Callers must not hold the
// user lock.
func (f *Forgejo) userExists(login string) bool {
	f.users.mu.Lock()
	defer f.users.mu.Unlock()
	if login == Poster {
		return true
	}
	_, ok := f.users.users[login]
	return ok
}

func (f *Forgejo) GetMyUserInfo(context.Context) (*forgejo_sdk.User, error) {
	f.users.mu.Lock()
	defer f.users.mu.Unlock()
	u := *f.users.add(Poster)
	return &u, nil
}

func (f *Forgejo) GetUserInfo(_ context.Context, login string) (*forgejo_sdk.User, error) {
	f.users.mu.Lock()
	defer f.users.mu.Unlock()
	if login == Poster {
		f.users.add(Poster)
	}
	u, ok := f.users.users[login]
	if !ok {
		return nil, notFound("user")
	}
	c := *u
	return &c, nil
}

// AddNotification files an unread (or read) notification for the user the fake
// acts as. The port can read and acknowledge notifications but not create them.
func (f *Forgejo) AddNotification(owner, repo, title string, unread bool) *forgejo_sdk.NotificationThread {
	f.users.mu.Lock()
	defer f.users.mu.Unlock()
	f.users.nextID++
	n := &forgejo_sdk.NotificationThread{
		ID:         f.users.nextID,
		Repository: &forgejo_sdk.Repository{Name: repo, FullName: owner + "/" + repo, Owner: user(owner)},
		Subject:    &forgejo_sdk.NotificationSubject{Title: title, Type: forgejo_sdk.NotifySubjectIssue, State: forgejo_sdk.NotifySubjectOpen},
		Unread:     unread,
		UpdatedAt:  f.Now(),
	}
	f.users.notifs = append(f.users.notifs, n)
	c := *n
	return &c
}

// matching returns the notifications that pass the filters, newest first, as
// copies. repo, when set, is "owner/repo".
func (s *userState) matching(repo string, status []forgejo_sdk.NotifyStatus, since, before time.Time) []*forgejo_sdk.NotificationThread {
	if len(status) == 0 {
		// Forgejo's default: what is waiting for the user.
		status = []forgejo_sdk.NotifyStatus{forgejo_sdk.NotifyStatusUnread}
	}
	want := map[forgejo_sdk.NotifyStatus]bool{}
	for _, st := range status {
		want[st] = true
	}
	var out []*forgejo_sdk.NotificationThread
	for _, n := range s.notifs {
		if repo != "" && n.Repository.FullName != repo {
			continue
		}
		st := forgejo_sdk.NotifyStatusRead
		if n.Unread {
			st = forgejo_sdk.NotifyStatusUnread
		}
		if !want[st] {
			continue
		}
		if !since.IsZero() && n.UpdatedAt.Before(since) {
			continue
		}
		if !before.IsZero() && !n.UpdatedAt.Before(before) {
			continue
		}
		c := *n
		out = append(out, &c)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].UpdatedAt.After(out[j].UpdatedAt)
		}
		return out[i].ID > out[j].ID
	})
	return out
}

func (f *Forgejo) ListNotifications(_ context.Context, opt forgejo_sdk.ListNotificationOptions) ([]*forgejo_sdk.NotificationThread, error) {
	return f.listNotifications("", opt), nil
}

func (f *Forgejo) ListRepoNotifications(_ context.Context, owner, repo string, opt forgejo_sdk.ListNotificationOptions) ([]*forgejo_sdk.NotificationThread, error) {
	return f.listNotifications(owner+"/"+repo, opt), nil
}

func (f *Forgejo) listNotifications(repo string, opt forgejo_sdk.ListNotificationOptions) []*forgejo_sdk.NotificationThread {
	f.users.mu.Lock()
	defer f.users.mu.Unlock()
	all := f.users.matching(repo, opt.Status, opt.Since, opt.Before)
	lo, hi := page(opt.ListOptions, len(all))
	return append([]*forgejo_sdk.NotificationThread{}, all[lo:hi]...)
}

func (f *Forgejo) notifThread(id int64) (*forgejo_sdk.NotificationThread, error) {
	for _, n := range f.users.notifs {
		if n.ID == id {
			return n, nil
		}
	}
	return nil, statusErr(http.StatusNotFound, "notification thread not found")
}

func (f *Forgejo) GetNotification(_ context.Context, id int64) (*forgejo_sdk.NotificationThread, error) {
	f.users.mu.Lock()
	defer f.users.mu.Unlock()
	n, err := f.notifThread(id)
	if err != nil {
		return nil, err
	}
	c := *n
	return &c, nil
}

func (f *Forgejo) ReadNotification(_ context.Context, id int64) (*forgejo_sdk.NotificationThread, error) {
	f.users.mu.Lock()
	defer f.users.mu.Unlock()
	n, err := f.notifThread(id)
	if err != nil {
		return nil, err
	}
	n.Unread = false
	c := *n
	return &c, nil
}

func (f *Forgejo) ReadNotifications(_ context.Context, opt forgejo_sdk.MarkNotificationOptions) error {
	return f.readNotifications("", opt)
}

func (f *Forgejo) ReadRepoNotifications(_ context.Context, owner, repo string, opt forgejo_sdk.MarkNotificationOptions) error {
	return f.readNotifications(owner+"/"+repo, opt)
}

func (f *Forgejo) readNotifications(repo string, opt forgejo_sdk.MarkNotificationOptions) error {
	f.users.mu.Lock()
	defer f.users.mu.Unlock()
	status := opt.Status
	for _, m := range f.users.matching(repo, status, time.Time{}, time.Time{}) {
		// LastReadAt bounds what is acknowledged: later notifications stay.
		if !opt.LastReadAt.IsZero() && m.UpdatedAt.After(opt.LastReadAt) {
			continue
		}
		n, _ := f.notifThread(m.ID)
		n.Unread = false
	}
	return nil
}
