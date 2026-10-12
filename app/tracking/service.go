// SPDX-License-Identifier: GPL-3.0-or-later

package tracking

import (
	"context"
	"fmt"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/list"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
)

// Service runs the time-tracking use cases. It holds only its Source; the port
// is looked up per call, after the input has been checked.
type Service struct{ src Source }

// New returns the use cases over src.
func New(src Source) *Service { return &Service{src: src} }

// IssueRef names an issue or pull request.
type IssueRef struct {
	Owner, Repo string
	Index       int64
}

// IssueTimesInput lists the tracked times of an issue. Since and Before are
// RFC 3339 text, or empty for no bound.
type IssueTimesInput struct {
	IssueRef
	Since, Before string
	Page, Limit   int
}

// RepoTimesInput lists the tracked times of a repository. User filters by
// user name.
type RepoTimesInput struct {
	Owner, Repo   string
	Since, Before string
	User          string
	Page, Limit   int
}

// AddTimeInput logs time against an issue. Exactly one of Seconds (nil when
// absent) and Duration (a Go duration such as "1h30m", empty when absent) must
// be given. CreatedAt is RFC 3339 text, or empty for now.
type AddTimeInput struct {
	IssueRef
	Seconds   *float64
	Duration  string
	CreatedAt string
	User      string
}

// IssueTimes returns the tracked times of an issue.
func (s *Service) IssueTimes(ctx context.Context, in IssueTimesInput) ([]*model.TrackedTime, error) {
	since, before, err := parseBounds(in.Since, in.Before)
	if err != nil {
		return nil, err
	}
	p, err := s.port(ctx)
	if err != nil {
		return nil, err
	}
	return window(ctx, p, list.Paging{Page: in.Page, Limit: in.Limit}, func(ctx context.Context, q list.Paging) ([]*model.TrackedTime, error) {
		times, err := p.ListIssueTrackedTimes(ctx, in.Owner, in.Repo, in.Index, TimeQuery{Since: since, Before: before, Page: q.Page, Limit: q.Limit})
		if err != nil {
			return nil, fmt.Errorf("list issue tracked times err: %w", err)
		}
		return times, nil
	})
}

// defaultLimit is the page size a tracked-time list uses when the caller gives
// none.
const defaultLimit = 20

// window reads one page of a tracked-time list through the list module: the
// instance ceiling applies, and a default limit is lowered to it.
func window(ctx context.Context, p Port, pg list.Paging, fetch func(context.Context, list.Paging) ([]*model.TrackedTime, error)) ([]*model.TrackedTime, error) {
	ceiling, known := p.MaxResponseItems(ctx)
	c := list.Ceiling{Max: ceiling, Known: known}
	return list.Items(ctx, list.Given(list.Paging(pg), defaultLimit, c), c,
		func(ctx context.Context, q list.Paging) (list.Batch[*model.TrackedTime], error) {
			times, err := fetch(ctx, q)
			return list.Batch[*model.TrackedTime]{Items: times}, err
		})
}

// RepoTimes returns the tracked times of a repository.
func (s *Service) RepoTimes(ctx context.Context, in RepoTimesInput) ([]*model.TrackedTime, error) {
	since, before, err := parseBounds(in.Since, in.Before)
	if err != nil {
		return nil, err
	}
	p, err := s.port(ctx)
	if err != nil {
		return nil, err
	}
	return window(ctx, p, list.Paging{Page: in.Page, Limit: in.Limit}, func(ctx context.Context, q list.Paging) ([]*model.TrackedTime, error) {
		times, err := p.ListRepoTrackedTimes(ctx, in.Owner, in.Repo, TimeQuery{Since: since, Before: before, User: in.User, Page: q.Page, Limit: q.Limit})
		if err != nil {
			return nil, fmt.Errorf("list repo tracked times err: %w", err)
		}
		return times, nil
	})
}

// MyTimes returns the tracked times of the authenticated user.
func (s *Service) MyTimes(ctx context.Context) ([]*model.TrackedTime, error) {
	p, err := s.port(ctx)
	if err != nil {
		return nil, err
	}
	times, err := p.ListMyTrackedTimes(ctx)
	if err != nil {
		return nil, fmt.Errorf("list my tracked times err: %w", err)
	}
	return times, nil
}

// AddTime logs time against an issue and returns the entry.
func (s *Service) AddTime(ctx context.Context, in AddTimeInput) (*model.TrackedTime, error) {
	seconds, err := resolveSeconds(in.Seconds, in.Duration)
	if err != nil {
		return nil, err
	}
	add := AddTime{Seconds: seconds, User: in.User}
	if in.CreatedAt != "" {
		if add.Created, err = time.Parse(time.RFC3339, in.CreatedAt); err != nil {
			return nil, &InvalidInputError{What: "invalid created_at (expected RFC3339)", Err: err}
		}
	}
	p, err := s.port(ctx)
	if err != nil {
		return nil, err
	}
	entry, err := p.AddIssueTime(ctx, in.Owner, in.Repo, in.Index, add)
	if err != nil {
		return nil, fmt.Errorf("add issue time err: %w", err)
	}
	return entry, nil
}

// ResetTime deletes every tracked time entry of an issue.
func (s *Service) ResetTime(ctx context.Context, ref IssueRef) error {
	p, err := s.port(ctx)
	if err != nil {
		return err
	}
	if err := p.ResetIssueTime(ctx, ref.Owner, ref.Repo, ref.Index); err != nil {
		return fmt.Errorf("reset issue time err: %w", err)
	}
	return nil
}

// DeleteTimeEntry deletes one tracked time entry of an issue.
func (s *Service) DeleteTimeEntry(ctx context.Context, ref IssueRef, timeID int64) error {
	p, err := s.port(ctx)
	if err != nil {
		return err
	}
	if err := p.DeleteIssueTimeEntry(ctx, ref.Owner, ref.Repo, ref.Index, timeID); err != nil {
		return fmt.Errorf("delete time entry err: %w", err)
	}
	return nil
}

// StartStopwatch starts the stopwatch of an issue.
func (s *Service) StartStopwatch(ctx context.Context, ref IssueRef) error {
	p, err := s.port(ctx)
	if err != nil {
		return err
	}
	if err := p.StartIssueStopwatch(ctx, ref.Owner, ref.Repo, ref.Index); err != nil {
		return fmt.Errorf("start issue stopwatch err: %w", err)
	}
	return nil
}

// StopStopwatch stops the stopwatch of an issue, recording its elapsed time.
func (s *Service) StopStopwatch(ctx context.Context, ref IssueRef) error {
	p, err := s.port(ctx)
	if err != nil {
		return err
	}
	if err := p.StopIssueStopwatch(ctx, ref.Owner, ref.Repo, ref.Index); err != nil {
		return fmt.Errorf("stop issue stopwatch err: %w", err)
	}
	return nil
}

// CancelStopwatch cancels the stopwatch of an issue without recording time.
func (s *Service) CancelStopwatch(ctx context.Context, ref IssueRef) error {
	p, err := s.port(ctx)
	if err != nil {
		return err
	}
	if err := p.CancelIssueStopwatch(ctx, ref.Owner, ref.Repo, ref.Index); err != nil {
		return fmt.Errorf("cancel issue stopwatch err: %w", err)
	}
	return nil
}

// MyStopwatches returns the running stopwatches of the authenticated user.
func (s *Service) MyStopwatches(ctx context.Context) ([]*model.StopWatch, error) {
	p, err := s.port(ctx)
	if err != nil {
		return nil, err
	}
	watches, err := p.ListMyStopwatches(ctx)
	if err != nil {
		return nil, fmt.Errorf("list my stopwatches err: %w", err)
	}
	return watches, nil
}

func (s *Service) port(ctx context.Context) (Port, error) { return s.src(ctx) }

// parseBounds reads the since and before filters, which are RFC 3339 or empty.
func parseBounds(since, before string) (time.Time, time.Time, error) {
	var s, b time.Time
	var err error
	if since != "" {
		if s, err = time.Parse(time.RFC3339, since); err != nil {
			return s, b, &InvalidInputError{What: "invalid since (expected RFC3339)", Err: err}
		}
	}
	if before != "" {
		if b, err = time.Parse(time.RFC3339, before); err != nil {
			return s, b, &InvalidInputError{What: "invalid before (expected RFC3339)", Err: err}
		}
	}
	return s, b, nil
}

// resolveSeconds picks the positive whole number of seconds to log from either
// the seconds or the duration. Exactly one must be given.
func resolveSeconds(seconds *float64, duration string) (int64, error) {
	hasDur := duration != ""
	switch {
	case seconds != nil && hasDur:
		return 0, fmt.Errorf("provide exactly one of 'seconds' or 'duration', not both")
	case seconds == nil && !hasDur:
		return 0, fmt.Errorf("one of 'seconds' or 'duration' is required")
	case seconds != nil:
		if *seconds <= 0 {
			return 0, fmt.Errorf("seconds must be positive, got %v", *seconds)
		}
		return int64(*seconds), nil
	default:
		d, err := time.ParseDuration(duration)
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q: %w", duration, err)
		}
		if d <= 0 {
			return 0, fmt.Errorf("duration must be positive, got %s", d)
		}
		return int64(d / time.Second), nil
	}
}
