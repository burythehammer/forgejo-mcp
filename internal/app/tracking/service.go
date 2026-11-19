// SPDX-License-Identifier: GPL-3.0-or-later

package tracking

import (
	"context"
	"fmt"
	"time"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/list"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/model"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/ptr"
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
// absent, otherwise a whole number of at least one) and Duration (a Go duration
// such as "1h30m", empty when absent) must be given. CreatedAt is RFC 3339
// text, or empty for now.
type AddTimeInput struct {
	IssueRef
	Seconds   *int64
	Duration  string
	CreatedAt string
	User      string
}

// IssueTimes returns one page of the tracked times of an issue with its next
// marker. Forgejo reports no total for these lists, so the next page is probed.
func (s *Service) IssueTimes(ctx context.Context, in IssueTimesInput) (model.TrackedTimePage, error) {
	since, before, err := parseBounds(in.Since, in.Before)
	if err != nil {
		return model.TrackedTimePage{}, err
	}
	p, err := s.port(ctx)
	if err != nil {
		return model.TrackedTimePage{}, err
	}
	c := ceiling(ctx, p)
	page, err := list.Run(ctx, list.NewRequest(list.Paging{Page: in.Page, Limit: in.Limit}, defaultLimit), c,
		func(ctx context.Context, q list.Paging) (list.Batch[*model.TrackedTime], error) {
			times, err := p.ListIssueTrackedTimes(ctx, in.Owner, in.Repo, in.Index, TimeQuery{Since: since, Before: before, Page: q.Page, Limit: q.Limit})
			if err != nil {
				return list.Batch[*model.TrackedTime]{}, fmt.Errorf("list issue tracked times err: %w", err)
			}
			return list.Batch[*model.TrackedTime]{Items: times}, nil
		})
	if err != nil {
		return model.TrackedTimePage{}, err
	}
	return timePage(page), nil
}

// defaultLimit is the page size a tracked-time list uses when the caller gives
// none.
const defaultLimit = 20

// RepoTimes returns one page of the tracked times of a repository with its next
// marker. Forgejo reports no total for these lists, so the next page is probed.
func (s *Service) RepoTimes(ctx context.Context, in RepoTimesInput) (model.TrackedTimePage, error) {
	since, before, err := parseBounds(in.Since, in.Before)
	if err != nil {
		return model.TrackedTimePage{}, err
	}
	p, err := s.port(ctx)
	if err != nil {
		return model.TrackedTimePage{}, err
	}
	c := ceiling(ctx, p)
	page, err := list.Run(ctx, list.NewRequest(list.Paging{Page: in.Page, Limit: in.Limit}, defaultLimit), c,
		func(ctx context.Context, q list.Paging) (list.Batch[*model.TrackedTime], error) {
			times, err := p.ListRepoTrackedTimes(ctx, in.Owner, in.Repo, TimeQuery{Since: since, Before: before, User: in.User, Page: q.Page, Limit: q.Limit})
			if err != nil {
				return list.Batch[*model.TrackedTime]{}, fmt.Errorf("list repo tracked times err: %w", err)
			}
			return list.Batch[*model.TrackedTime]{Items: times}, nil
		})
	if err != nil {
		return model.TrackedTimePage{}, err
	}
	return timePage(page), nil
}

// ceiling reads the instance's pagination ceiling through the port the call
// already holds.
func ceiling(ctx context.Context, src list.CeilingSource) list.Ceiling {
	max, known := src.MaxResponseItems(ctx)
	return list.Ceiling{Max: max, Known: known}
}

// timePage renders one fetched page; an empty page is an empty list, not null.
func timePage(page list.Bounded[*model.TrackedTime]) model.TrackedTimePage {
	times := page.Items
	if times == nil {
		times = []*model.TrackedTime{}
	}
	return model.TrackedTimePage{Times: times, PageMeta: model.PageMetaOf(page)}
}

// MyTimesInput pages the authenticated user's tracked times. Page and Limit are
// 1-based; a zero Limit takes the default.
type MyTimesInput struct{ Page, Limit int }

// MyTimes returns one page of the tracked times of the authenticated user with
// its next marker. The port reads the whole list, so it is cut here, and no
// total is reported.
func (s *Service) MyTimes(ctx context.Context, in MyTimesInput) (model.TrackedTimePage, error) {
	p, err := s.port(ctx)
	if err != nil {
		return model.TrackedTimePage{}, err
	}
	c := ceiling(ctx, p)
	page, err := list.Run(ctx, list.NewRequest(list.Paging{Page: in.Page, Limit: in.Limit}, defaultLimit), c,
		func(ctx context.Context, q list.Paging) (list.Batch[*model.TrackedTime], error) {
			all, err := p.ListMyTrackedTimes(ctx)
			if err != nil {
				return list.Batch[*model.TrackedTime]{}, fmt.Errorf("list my tracked times err: %w", err)
			}
			rows := list.Cut(all, q).Items
			return list.Batch[*model.TrackedTime]{Items: rows, Total: ptr.To(len(all))}, nil
		})
	if err != nil {
		return model.TrackedTimePage{}, err
	}
	out := timePage(page)
	out.TotalCount = nil
	return out, nil
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

// MyStopwatchesInput pages the authenticated user's running stopwatches. Page
// and Limit are 1-based; a zero Limit takes the default.
type MyStopwatchesInput struct{ Page, Limit int }

// MyStopwatches returns one page of the running stopwatches of the
// authenticated user with its next marker. Forgejo does not page the endpoint,
// so the list is cut here and its length is the grand total.
func (s *Service) MyStopwatches(ctx context.Context, in MyStopwatchesInput) (model.StopWatchPage, error) {
	p, err := s.port(ctx)
	if err != nil {
		return model.StopWatchPage{}, err
	}
	c := ceiling(ctx, p)
	page, err := list.Run(ctx, list.NewRequest(list.Paging{Page: in.Page, Limit: in.Limit}, defaultLimit), c,
		func(ctx context.Context, q list.Paging) (list.Batch[*model.StopWatch], error) {
			all, err := p.ListMyStopwatches(ctx)
			if err != nil {
				return list.Batch[*model.StopWatch]{}, fmt.Errorf("list my stopwatches err: %w", err)
			}
			rows := list.Cut(all, q).Items
			return list.Batch[*model.StopWatch]{Items: rows, Total: ptr.To(len(all))}, nil
		})
	if err != nil {
		return model.StopWatchPage{}, err
	}
	watches := page.Items
	if watches == nil {
		watches = []*model.StopWatch{}
	}
	out := model.StopWatchPage{Stopwatches: watches, PageMeta: model.PageMetaOf(page)}
	// Batch.Total above is a local count that only lets list.Run decide has_next.
	// It is not the server's grand total (the endpoint sends no X-Total-Count),
	// and total_count is reserved for that grand total, so it stays omitted.
	out.TotalCount = nil
	return out, nil
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

// resolveSeconds picks the whole number of seconds to log from either the
// seconds or the duration. Exactly one must be given. The seconds arrive already
// validated as a whole number of at least one (see the numeric-argument module),
// so only a duration string is parsed and checked here.
func resolveSeconds(seconds *int64, duration string) (int64, error) {
	hasDur := duration != ""
	switch {
	case seconds != nil && hasDur:
		return 0, fmt.Errorf("provide exactly one of 'seconds' or 'duration', not both")
	case seconds == nil && !hasDur:
		return 0, fmt.Errorf("one of 'seconds' or 'duration' is required")
	case seconds != nil:
		return *seconds, nil
	default:
		d, err := time.ParseDuration(duration)
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q: %w", duration, err)
		}
		whole := int64(d / time.Second)
		if whole < 1 {
			return 0, fmt.Errorf("duration must be at least one second, got %s", d)
		}
		return whole, nil
	}
}
