// SPDX-License-Identifier: GPL-3.0-or-later

package forge

import (
	"context"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// Tracking is the time-tracking domain of the port: tracked time entries and
// issue stopwatches. Failures from Forgejo are *StatusError.
type Tracking interface {
	ListIssueTrackedTimes(ctx context.Context, owner, repo string, index int64, opt forgejo_sdk.ListTrackedTimesOptions) ([]*forgejo_sdk.TrackedTime, error)
	ListRepoTrackedTimes(ctx context.Context, owner, repo string, opt forgejo_sdk.ListTrackedTimesOptions) ([]*forgejo_sdk.TrackedTime, error)
	ListMyTrackedTimes(ctx context.Context) ([]*forgejo_sdk.TrackedTime, error)
	AddIssueTime(ctx context.Context, owner, repo string, index int64, opt forgejo_sdk.AddTimeOption) (*forgejo_sdk.TrackedTime, error)
	ResetIssueTime(ctx context.Context, owner, repo string, index int64) error
	DeleteIssueTimeEntry(ctx context.Context, owner, repo string, index, timeID int64) error

	StartIssueStopwatch(ctx context.Context, owner, repo string, index int64) error
	// StopIssueStopwatch stops the stopwatch and records its elapsed time as a
	// tracked time entry.
	StopIssueStopwatch(ctx context.Context, owner, repo string, index int64) error
	CancelIssueStopwatch(ctx context.Context, owner, repo string, index int64) error
	ListMyStopwatches(ctx context.Context) ([]*forgejo_sdk.StopWatch, error)
}
