// SPDX-License-Identifier: GPL-3.0-or-later

package tracking_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
	apptracking "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/tracking"
)

// The use cases run against a stub port that holds fixed, repo-owned data and
// records what it was asked: no MCP, no SDK, no HTTP. Each test builds its own,
// so tests run in parallel.

type stubPort struct {
	err   error
	last  string
	query apptracking.TimeQuery
	add   apptracking.AddTime
}

func (p *stubPort) ListIssueTrackedTimes(_ context.Context, _, _ string, _ int64, q apptracking.TimeQuery) ([]*model.TrackedTime, error) {
	p.last, p.query = "issue", q
	return []*model.TrackedTime{{ID: 1, Time: 90}}, p.err
}

func (p *stubPort) ListRepoTrackedTimes(_ context.Context, _, _ string, q apptracking.TimeQuery) ([]*model.TrackedTime, error) {
	p.last, p.query = "repo", q
	return nil, p.err
}

func (p *stubPort) ListMyTrackedTimes(context.Context) ([]*model.TrackedTime, error) {
	p.last = "mine"
	return nil, p.err
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
	if p.query.Since.IsZero() || p.query.Before.IsZero() || p.query.Page != 2 || p.query.Limit != 5 || p.query.User != "" {
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
		{"my times", func(s *apptracking.Service) error { _, err := s.MyTimes(context.Background()); return err }, "list my tracked times err: "},
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
	if _, err := s.MyTimes(context.Background()); !errors.Is(err, denied) || err.Error() != denied.Error() {
		t.Fatalf("got %v", err)
	}
}
