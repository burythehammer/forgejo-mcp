// SPDX-License-Identifier: GPL-3.0-or-later

// Package trackingadapter implements the time-tracking use cases' port
// (app/tracking.Port) over the Forgejo port (forge.Tracking). It is the one
// place that translates between repo-owned types and SDK types for the
// time-tracking domain: inputs become SDK options, SDK results become app/model
// values. Results are converted through their JSON form, which is exactly what
// a tool renders, so a field the repo-owned type shares by JSON name cannot
// drift in value.
package trackingadapter

import (
	"context"
	"encoding/json"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
	apptracking "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/tracking"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// New returns the use cases' port over t.
func New(t forge.Tracking) apptracking.Port { return adapter{t: t} }

type adapter struct{ t forge.Tracking }

var _ apptracking.Port = adapter{}

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

func listOptions(q apptracking.TimeQuery) forgejo_sdk.ListOptions {
	return forgejo_sdk.ListOptions{Page: q.Page, PageSize: q.Limit}
}

func (a adapter) ListIssueTrackedTimes(ctx context.Context, owner, repo string, index int64, q apptracking.TimeQuery) ([]*model.TrackedTime, error) {
	// The user filter is ignored by the issue-level endpoint, so it is not sent.
	times, err := a.t.ListIssueTrackedTimes(ctx, owner, repo, index, forgejo_sdk.ListTrackedTimesOptions{
		ListOptions: listOptions(q), Since: q.Since, Before: q.Before,
	})
	return convert[[]*model.TrackedTime](times, err)
}

func (a adapter) ListRepoTrackedTimes(ctx context.Context, owner, repo string, q apptracking.TimeQuery) ([]*model.TrackedTime, error) {
	times, err := a.t.ListRepoTrackedTimes(ctx, owner, repo, forgejo_sdk.ListTrackedTimesOptions{
		ListOptions: listOptions(q), Since: q.Since, Before: q.Before, User: q.User,
	})
	return convert[[]*model.TrackedTime](times, err)
}

func (a adapter) ListMyTrackedTimes(ctx context.Context) ([]*model.TrackedTime, error) {
	times, err := a.t.ListMyTrackedTimes(ctx)
	return convert[[]*model.TrackedTime](times, err)
}

func (a adapter) AddIssueTime(ctx context.Context, owner, repo string, index int64, in apptracking.AddTime) (*model.TrackedTime, error) {
	entry, err := a.t.AddIssueTime(ctx, owner, repo, index, forgejo_sdk.AddTimeOption{
		Time: in.Seconds, Created: in.Created, User: in.User,
	})
	return convert[*model.TrackedTime](entry, err)
}

func (a adapter) ResetIssueTime(ctx context.Context, owner, repo string, index int64) error {
	return a.t.ResetIssueTime(ctx, owner, repo, index)
}

func (a adapter) DeleteIssueTimeEntry(ctx context.Context, owner, repo string, index, timeID int64) error {
	return a.t.DeleteIssueTimeEntry(ctx, owner, repo, index, timeID)
}

func (a adapter) StartIssueStopwatch(ctx context.Context, owner, repo string, index int64) error {
	return a.t.StartIssueStopwatch(ctx, owner, repo, index)
}

func (a adapter) StopIssueStopwatch(ctx context.Context, owner, repo string, index int64) error {
	return a.t.StopIssueStopwatch(ctx, owner, repo, index)
}

func (a adapter) CancelIssueStopwatch(ctx context.Context, owner, repo string, index int64) error {
	return a.t.CancelIssueStopwatch(ctx, owner, repo, index)
}

func (a adapter) ListMyStopwatches(ctx context.Context) ([]*model.StopWatch, error) {
	watches, err := a.t.ListMyStopwatches(ctx)
	return convert[[]*model.StopWatch](watches, err)
}
