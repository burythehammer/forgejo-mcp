// SPDX-License-Identifier: GPL-3.0-or-later

package tracking_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	apptracking "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/tracking"
)

// The use cases run against a stub port that holds fixed, repo-owned data and
// records what it was asked: no MCP, no SDK, no HTTP. Each test builds its own,
// so tests run in parallel.

type stubPort struct {
	err   error
	last  string
	query apptracking.TimeQuery
	add   apptracking.AddTime
	// times are the rows the server holds; a list is served one page of them.
	times []*model.TrackedTime
}

// servePage returns the rows a server-paged list sends for q.
func servePage(all []*model.TrackedTime, q apptracking.TimeQuery) []*model.TrackedTime {
	lo := (q.Page - 1) * q.Limit
	if lo < 0 || lo > len(all) {
		lo = len(all)
	}
	return all[lo:min(lo+q.Limit, len(all))]
}

func (p *stubPort) ListIssueTrackedTimes(_ context.Context, _, _ string, _ int64, q apptracking.TimeQuery) ([]*model.TrackedTime, error) {
	p.last, p.query = "issue", q
	if p.times != nil {
		return servePage(p.times, q), p.err
	}
	return []*model.TrackedTime{{ID: 1, Time: 90}}, p.err
}

func (p *stubPort) ListRepoTrackedTimes(_ context.Context, _, _ string, q apptracking.TimeQuery) ([]*model.TrackedTime, error) {
	p.last, p.query = "repo", q
	return servePage(p.times, q), p.err
}

func (p *stubPort) ListMyTrackedTimes(context.Context) ([]*model.TrackedTime, error) {
	p.last = "mine"
	return p.times, p.err
}

func (p *stubPort) AddIssueTime(_ context.Context, _, _ string, _ int64, in apptracking.AddTime) (*model.TrackedTime, error) {
	p.last, p.add = "add", in
	return &model.TrackedTime{ID: 2, Time: in.Seconds}, p.err
}

func (p *stubPort) ResetIssueTime(context.Context, string, string, int64) error {
	p.last = "reset"
	return p.err
}

func (p *stubPort) DeleteIssueTimeEntry(context.Context, string, string, int64, int64) error {
	p.last = "delete"
	return p.err
}

func (p *stubPort) StartIssueStopwatch(context.Context, string, string, int64) error {
	p.last = "start"
	return p.err
}

func (p *stubPort) StopIssueStopwatch(context.Context, string, string, int64) error {
	p.last = "stop"
	return p.err
}

func (p *stubPort) CancelIssueStopwatch(context.Context, string, string, int64) error {
	p.last = "cancel"
	return p.err
}

func (p *stubPort) ListMyStopwatches(context.Context) ([]*model.StopWatch, error) {
	p.last = "watches"
	return []*model.StopWatch{{RepoName: "hello"}}, p.err
}

// service returns the use cases over p, and counts how often the port was
// looked up, so a test can see whether a request got as far as Forgejo.
func service(p *stubPort, lookups *int) *apptracking.Service {
	return apptracking.New(func(context.Context) (apptracking.Port, error) {
		*lookups++
		return p, nil
	})
}

var ref = apptracking.IssueRef{Owner: "octo", Repo: "hello", Index: 7}

func seconds(v float64) *float64 { return &v }

func TestResolveSecondsRules(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		seconds  *float64
		duration string
		want     int64
		wantErr  string
	}{
		{name: "seconds", seconds: seconds(900), want: 900},
		{name: "fraction truncates", seconds: seconds(2.7), want: 2},
		{name: "duration 15m", duration: "15m", want: 900},
		{name: "duration 1h30m", duration: "1h30m", want: 5400},
		{name: "both rejected", seconds: seconds(60), duration: "1m", wantErr: "exactly one"},
		{name: "neither rejected", wantErr: "required"},
		{name: "zero seconds", seconds: seconds(0), wantErr: "positive"},
		{name: "negative seconds", seconds: seconds(-5), wantErr: "positive"},
		{name: "negative duration", duration: "-30s", wantErr: "positive"},
		{name: "malformed duration", duration: "not a duration", wantErr: "invalid duration"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := &stubPort{}
			lookups := 0
			_, err := service(p, &lookups).AddTime(context.Background(), apptracking.AddTimeInput{IssueRef: ref, Seconds: tc.seconds, Duration: tc.duration})
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
				}
				if lookups != 0 {
					t.Fatalf("an invalid request reached the port")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if p.add.Seconds != tc.want {
				t.Fatalf("logged %d seconds, want %d", p.add.Seconds, tc.want)
			}
		})
	}
}

func TestInvalidInputIsCheckedBeforeTheCredentialIsLookedUp(t *testing.T) {
	t.Parallel()
	p := &stubPort{}
	lookups := 0
	s := apptracking.New(func(context.Context) (apptracking.Port, error) {
		lookups++
		return nil, errors.New("no credential")
	})
	_, err := s.AddTime(context.Background(), apptracking.AddTimeInput{IssueRef: ref, Duration: "bogus"})
	if err == nil || !strings.HasPrefix(err.Error(), "invalid duration") {
		t.Fatalf("got %v", err)
	}
	if _, err := s.IssueTimes(context.Background(), apptracking.IssueTimesInput{IssueRef: ref, Since: "yesterday"}); err == nil || !strings.HasPrefix(err.Error(), "invalid since (expected RFC3339)") {
		t.Fatalf("got %v", err)
	}
	if lookups != 0 {
		t.Fatalf("validation failures looked up the credential %d times", lookups)
	}
	if p.last != "" {
		t.Fatalf("port was called: %s", p.last)
	}
}

func TestCreatedAtIsParsed(t *testing.T) {
	t.Parallel()
	p := &stubPort{}
	lookups := 0
	if _, err := service(p, &lookups).AddTime(context.Background(), apptracking.AddTimeInput{IssueRef: ref, Seconds: seconds(5), CreatedAt: "2026-01-02T03:04:05Z", User: "bob"}); err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if !p.add.Created.Equal(want) || p.add.User != "bob" {
		t.Fatalf("add %+v", p.add)
	}
	_, err := service(p, &lookups).AddTime(context.Background(), apptracking.AddTimeInput{IssueRef: ref, Seconds: seconds(5), CreatedAt: "tomorrow"})
	if err == nil || !strings.HasPrefix(err.Error(), "invalid created_at (expected RFC3339): ") {
		t.Fatalf("got %v", err)
	}
}

func TestIssueTimesFilterAndPageWithoutUser(t *testing.T) {
	t.Parallel()
	p := &stubPort{}
	lookups := 0
	_, err := service(p, &lookups).IssueTimes(context.Background(), apptracking.IssueTimesInput{
		IssueRef: ref, Since: "2026-01-01T00:00:00Z", Before: "2026-02-01T00:00:00Z", Page: 2, Limit: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	// The requested page is fetched, then the next one is probed for has_next,
	// so the last request the port saw is page 3.
	if p.query.Since.IsZero() || p.query.Before.IsZero() || p.query.Page != 3 || p.query.Limit != 5 || p.query.User != "" {
		t.Fatalf("query %+v", p.query)
	}
}

func TestRepoTimesCarryTheUserFilter(t *testing.T) {
	t.Parallel()
	p := &stubPort{}
	lookups := 0
	if _, err := service(p, &lookups).RepoTimes(context.Background(), apptracking.RepoTimesInput{Owner: "octo", Repo: "hello", User: "bob", Page: 1, Limit: 20}); err != nil {
		t.Fatal(err)
	}
	if p.query.User != "bob" {
		t.Fatalf("query %+v", p.query)
	}
}

func TestFailuresAreWrapped(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	tests := []struct {
		name   string
		call   func(*apptracking.Service) error
		prefix string
	}{
		{"issue times", func(s *apptracking.Service) error {
			_, err := s.IssueTimes(context.Background(), apptracking.IssueTimesInput{IssueRef: ref})
			return err
		}, "list issue tracked times err: "},
		{"repo times", func(s *apptracking.Service) error {
			_, err := s.RepoTimes(context.Background(), apptracking.RepoTimesInput{})
			return err
		}, "list repo tracked times err: "},
		{"my times", func(s *apptracking.Service) error {
			_, err := s.MyTimes(context.Background(), apptracking.MyTimesInput{})
			return err
		}, "list my tracked times err: "},
		{"add", func(s *apptracking.Service) error {
			_, err := s.AddTime(context.Background(), apptracking.AddTimeInput{IssueRef: ref, Seconds: seconds(5)})
			return err
		}, "add issue time err: "},
		{"reset", func(s *apptracking.Service) error { return s.ResetTime(context.Background(), ref) }, "reset issue time err: "},
		{"delete", func(s *apptracking.Service) error { return s.DeleteTimeEntry(context.Background(), ref, 3) }, "delete time entry err: "},
		{"start", func(s *apptracking.Service) error { return s.StartStopwatch(context.Background(), ref) }, "start issue stopwatch err: "},
		{"stop", func(s *apptracking.Service) error { return s.StopStopwatch(context.Background(), ref) }, "stop issue stopwatch err: "},
		{"cancel", func(s *apptracking.Service) error { return s.CancelStopwatch(context.Background(), ref) }, "cancel issue stopwatch err: "},
		{"watches", func(s *apptracking.Service) error { _, err := s.MyStopwatches(context.Background()); return err }, "list my stopwatches err: "},
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

func TestSourceFailureIsReturnedAsIs(t *testing.T) {
	t.Parallel()
	denied := errors.New("no credential")
	s := apptracking.New(func(context.Context) (apptracking.Port, error) { return nil, denied })
	if _, err := s.MyTimes(context.Background(), apptracking.MyTimesInput{}); !errors.Is(err, denied) || err.Error() != denied.Error() {
		t.Fatalf("got %v", err)
	}
}

// MaxResponseItems reports an unknown ceiling for the test double.
func (*stubPort) MaxResponseItems(context.Context) (int, bool) { return 0, false }

// TestTimesEnvelopeSaysWhetherMoreFollow covers the four shapes a tracked-time
// page can take. Forgejo reports no grand total for these lists, so has_next is
// settled by one probe of the next page: a full page may be the last one.
func TestTimesEnvelopeSaysWhetherMoreFollow(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		n           int
		page, limit int
		count       int
		hasNext     bool
	}{
		{"full page with more", 5, 1, 2, 2, true},
		{"short last page", 5, 3, 2, 1, false},
		{"exact multiple last page", 4, 2, 2, 2, false},
		{"full page before exact end", 4, 1, 2, 2, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			times := make([]*model.TrackedTime, tc.n)
			for i := range times {
				times[i] = &model.TrackedTime{ID: int64(i + 1)}
			}
			p := &stubPort{times: times}
			lookups := 0
			svc := service(p, &lookups)
			issue, err := svc.IssueTimes(context.Background(), apptracking.IssueTimesInput{IssueRef: ref, Page: tc.page, Limit: tc.limit})
			if err != nil {
				t.Fatal(err)
			}
			repo, err := svc.RepoTimes(context.Background(), apptracking.RepoTimesInput{Owner: "octo", Repo: "hello", Page: tc.page, Limit: tc.limit})
			if err != nil {
				t.Fatal(err)
			}
			for _, got := range []model.TrackedTimePage{issue, repo} {
				if got.Count != tc.count || len(got.Times) != tc.count || got.HasNext != tc.hasNext || got.Page != tc.page || got.Limit != tc.limit {
					t.Fatalf("page %+v, want count %d has_next %v page %d limit %d", got, tc.count, tc.hasNext, tc.page, tc.limit)
				}
			}
		})
	}
}
