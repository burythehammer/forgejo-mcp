// SPDX-License-Identifier: GPL-3.0-or-later

package user_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
	appuser "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/user"
)

// The use cases run against a stub port that holds fixed, repo-owned data and
// records what it was asked: no MCP, no SDK, no HTTP. Each test builds its own,
// so tests run in parallel.

type stubPort struct {
	err     error
	last    string
	query   appuser.NotificationQuery
	read    appuser.ReadQuery
	threads []*model.NotificationThread
	// firstRepo is the query of the first repo-list call; query is the last one,
	// which is the probe for the next page when the first page was full.
	firstRepo appuser.NotificationQuery
	repoCalls int
}

func (p *stubPort) GetMyUserInfo(context.Context) (*model.User, error) {
	p.last = "me"
	return &model.User{UserName: "octo"}, p.err
}

func (p *stubPort) ListNotifications(_ context.Context, q appuser.NotificationQuery) ([]*model.NotificationThread, error) {
	p.last, p.query = "list", q
	return p.threads, p.err
}

func (p *stubPort) ListRepoNotifications(_ context.Context, _, _ string, q appuser.NotificationQuery) ([]*model.NotificationThread, error) {
	p.last, p.query = "repo list", q
	if p.repoCalls == 0 {
		p.firstRepo = q
	}
	p.repoCalls++
	return p.threads, p.err
}

func (p *stubPort) GetNotification(_ context.Context, id int64) (*model.NotificationThread, error) {
	p.last = "thread"
	return &model.NotificationThread{ID: id}, p.err
}

func (p *stubPort) ReadNotification(_ context.Context, id int64) (*model.NotificationThread, error) {
	p.last = "mark"
	return &model.NotificationThread{ID: id}, p.err
}

func (p *stubPort) ReadNotifications(_ context.Context, in appuser.ReadQuery) error {
	p.last, p.read = "mark all", in
	return p.err
}

func (p *stubPort) ReadRepoNotifications(_ context.Context, _, _ string, in appuser.ReadQuery) error {
	p.last, p.read = "mark repo", in
	return p.err
}

// service returns the use cases over p, and counts how often the port was
// looked up, so a test can see whether a request got as far as Forgejo.
func service(p *stubPort, lookups *int) *appuser.Service {
	return appuser.New(func(context.Context) (appuser.Port, error) {
		*lookups++
		return p, nil
	})
}

func TestUnreadOnlyUnlessAll(t *testing.T) {
	t.Parallel()
	p := &stubPort{}
	lookups := 0
	s := service(p, &lookups)
	if _, err := s.Notifications(context.Background(), appuser.NotificationsInput{Page: 1, Limit: 20}); err != nil {
		t.Fatal(err)
	}
	if !p.query.UnreadOnly {
		t.Fatalf("default must be unread only: %+v", p.query)
	}
	if _, err := s.Notifications(context.Background(), appuser.NotificationsInput{All: true, Page: 1, Limit: 20}); err != nil {
		t.Fatal(err)
	}
	if p.query.UnreadOnly {
		t.Fatalf("all must include read threads: %+v", p.query)
	}
}

func TestTimesAreParsedAndPagesPassThrough(t *testing.T) {
	t.Parallel()
	p := &stubPort{}
	lookups := 0
	_, err := service(p, &lookups).RepoNotifications(context.Background(), appuser.RepoNotificationsInput{
		Owner: "octo", Repo: "hello",
		NotificationsInput: appuser.NotificationsInput{Since: "2026-01-01T00:00:00Z", Before: "2026-02-01T00:00:00Z", Page: 3, Limit: 9},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !p.firstRepo.Since.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) || p.firstRepo.Page != 3 || p.firstRepo.Limit != 9 {
		t.Fatalf("first query %+v", p.firstRepo)
	}
}

func TestBadTimesAreRefusedBeforeTheCredentialIsLookedUp(t *testing.T) {
	t.Parallel()
	s := appuser.New(func(context.Context) (appuser.Port, error) { return nil, errors.New("no credential") })
	_, err := s.Notifications(context.Background(), appuser.NotificationsInput{Since: "yesterday", Page: 1, Limit: 20})
	if err == nil || !strings.HasPrefix(err.Error(), "invalid since time format (expected RFC3339): ") {
		t.Fatalf("since: %v", err)
	}
	_, err = s.Notifications(context.Background(), appuser.NotificationsInput{Before: "soon", Page: 1, Limit: 20})
	if err == nil || !strings.HasPrefix(err.Error(), "invalid before time format (expected RFC3339): ") {
		t.Fatalf("before: %v", err)
	}
	err = s.MarkAllRead(context.Background(), "later")
	if err == nil || !strings.HasPrefix(err.Error(), "invalid last_read_at time format (expected RFC3339): ") {
		t.Fatalf("last_read_at: %v", err)
	}
	err = s.MarkRepoRead(context.Background(), "octo", "hello", "later")
	if err == nil || !strings.HasPrefix(err.Error(), "invalid last_read_at time format (expected RFC3339): ") {
		t.Fatalf("repo last_read_at: %v", err)
	}
}

func TestLastReadAtIsPassedOn(t *testing.T) {
	t.Parallel()
	p := &stubPort{}
	lookups := 0
	s := service(p, &lookups)
	if err := s.MarkAllRead(context.Background(), "2026-01-15T10:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if !p.read.LastReadAt.Equal(time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("read %+v", p.read)
	}
	if err := s.MarkRepoRead(context.Background(), "octo", "hello", ""); err != nil {
		t.Fatal(err)
	}
	if !p.read.LastReadAt.IsZero() {
		t.Fatalf("empty last_read_at must mean now (zero), got %+v", p.read)
	}
}

func TestFailuresAreWrapped(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	tests := []struct {
		name   string
		call   func(*appuser.Service) error
		prefix string
	}{
		{"my info", func(s *appuser.Service) error { _, err := s.MyInfo(context.Background()); return err }, "get user info err: "},
		{"notifications", func(s *appuser.Service) error {
			_, err := s.Notifications(context.Background(), appuser.NotificationsInput{Page: 1, Limit: 20})
			return err
		}, "list notifications err: "},
		{"repo notifications", func(s *appuser.Service) error {
			_, err := s.RepoNotifications(context.Background(), appuser.RepoNotificationsInput{Owner: "o", Repo: "r"})
			return err
		}, "list repo notifications err: "},
		{"thread", func(s *appuser.Service) error { _, err := s.Thread(context.Background(), 1); return err }, "get notification thread err: "},
		{"mark", func(s *appuser.Service) error { _, err := s.MarkRead(context.Background(), 1); return err }, "mark notification read err: "},
		{"mark all", func(s *appuser.Service) error { return s.MarkAllRead(context.Background(), "") }, "mark all notifications read err: "},
		{"mark repo", func(s *appuser.Service) error { return s.MarkRepoRead(context.Background(), "o", "r", "") }, "mark repo notifications read err: "},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			lookups := 0
			err := tc.call(service(&stubPort{err: boom}, &lookups))
			if err == nil || !strings.HasPrefix(err.Error(), tc.prefix) || !errors.Is(err, boom) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

// MaxResponseItems reports an unknown ceiling for the test double.
func (*stubPort) MaxResponseItems(context.Context) (int, bool) { return 0, false }

// pagedRepoPort serves n notifications of one repository, sliced by the page
// and limit the use case passes, so a test sees the window it asked for.
type pagedRepoPort struct {
	stubPort
	n int
}

func (p *pagedRepoPort) ListRepoNotifications(_ context.Context, _, _ string, q appuser.NotificationQuery) ([]*model.NotificationThread, error) {
	var out []*model.NotificationThread
	for i := (q.Page - 1) * q.Limit; i < q.Page*q.Limit && i < p.n; i++ {
		out = append(out, &model.NotificationThread{ID: int64(i + 1)})
	}
	return out, nil
}

func TestRepoNotificationsEnvelopeNamesTheNextPage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name          string
		n, page, want int
		hasNext       bool
	}{
		{"full page with more threads", 3, 1, 2, true},
		{"short last page", 3, 2, 1, false},
		{"exact multiple, full last page", 4, 2, 2, false},
		{"past the end", 4, 3, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := appuser.New(func(context.Context) (appuser.Port, error) {
				return &pagedRepoPort{n: c.n}, nil
			})
			got, err := s.RepoNotifications(context.Background(), appuser.RepoNotificationsInput{
				Owner: "octo", Repo: "hello",
				NotificationsInput: appuser.NotificationsInput{Page: c.page, Limit: 2},
			})
			if err != nil || len(got.Notifications) != c.want || got.Count != c.want || got.HasNext != c.hasNext || got.Page != c.page || got.Limit != 2 {
				t.Fatalf("got %+v, %v", got, err)
			}
		})
	}
}

// pagedNotificationsPort serves n notifications, sliced by the page and limit
// the use case passes.
type pagedNotificationsPort struct {
	stubPort
	n int
}

func (p *pagedNotificationsPort) ListNotifications(_ context.Context, q appuser.NotificationQuery) ([]*model.NotificationThread, error) {
	var out []*model.NotificationThread
	for i := (q.Page - 1) * q.Limit; i < q.Page*q.Limit && i < p.n; i++ {
		out = append(out, &model.NotificationThread{ID: int64(i + 1)})
	}
	return out, nil
}

func TestNotificationsEnvelopeNamesTheNextPage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name          string
		n, page, want int
		hasNext       bool
	}{
		{"full page with more threads", 3, 1, 2, true},
		{"short last page", 3, 2, 1, false},
		{"exact multiple, full last page", 4, 2, 2, false},
		{"past the end", 4, 3, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			s := appuser.New(func(context.Context) (appuser.Port, error) {
				return &pagedNotificationsPort{n: c.n}, nil
			})
			got, err := s.Notifications(context.Background(), appuser.NotificationsInput{Page: c.page, Limit: 2})
			if err != nil || len(got.Notifications) != c.want || got.Count != c.want || got.HasNext != c.hasNext || got.Page != c.page || got.Limit != 2 {
				t.Fatalf("got %+v, %v", got, err)
			}
		})
	}
}
