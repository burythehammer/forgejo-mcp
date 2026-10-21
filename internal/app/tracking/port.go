// SPDX-License-Identifier: GPL-3.0-or-later

// Package tracking holds the time-tracking use cases behind the tracked-time
// and stopwatch tools. Like all of internal/app/ it is MCP-free and SDK-free
// (scripts/ci/check-app-boundary.sh): use cases take typed inputs, reach
// Forgejo only through Port, and return repo-owned types from internal/app/model for an
// MCP adapter in internal/operation/tracking to render.
package tracking

import (
	"context"
	"time"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
)

// Port is the time-tracking domain as the use cases need it, in repo-owned
// types. internal/forge/trackingadapter implements it over forge.Tracking.
type Port interface {
	// MaxResponseItems is the instance's pagination ceiling; ok is false when it
	// cannot be determined, which is "unknown", never a ceiling of zero.
	MaxResponseItems(ctx context.Context) (limit int, ok bool)
	ListIssueTrackedTimes(ctx context.Context, owner, repo string, index int64, q TimeQuery) ([]*model.TrackedTime, error)
	ListRepoTrackedTimes(ctx context.Context, owner, repo string, q TimeQuery) ([]*model.TrackedTime, error)
	ListMyTrackedTimes(ctx context.Context) ([]*model.TrackedTime, error)
	AddIssueTime(ctx context.Context, owner, repo string, index int64, in AddTime) (*model.TrackedTime, error)
	ResetIssueTime(ctx context.Context, owner, repo string, index int64) error
	DeleteIssueTimeEntry(ctx context.Context, owner, repo string, index, timeID int64) error
	StartIssueStopwatch(ctx context.Context, owner, repo string, index int64) error
	StopIssueStopwatch(ctx context.Context, owner, repo string, index int64) error
	CancelIssueStopwatch(ctx context.Context, owner, repo string, index int64) error
	ListMyStopwatches(ctx context.Context) ([]*model.StopWatch, error)
}

// Source yields the port for one request. The use cases call it only after
// they have checked their input, so a request that is invalid is refused the
// same way whether or not the request's credential resolves. The operation
// layer passes the credential policy's lookup here.
type Source func(ctx context.Context) (Port, error)

// TimeQuery filters and pages a list of tracked times. Zero Since and Before
// mean no bound; User is only used by the repository-level list.
type TimeQuery struct {
	Since, Before time.Time
	User          string
	Page, Limit   int
}

// AddTime is a time entry to log. Zero Created means now; empty User means the
// authenticated user.
type AddTime struct {
	Seconds int64
	Created time.Time
	User    string
}
