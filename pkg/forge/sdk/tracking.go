// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var _ forge.Tracking = (*Adapter)(nil)

func (a *Adapter) ListIssueTrackedTimes(_ context.Context, owner, repo string, index int64, opt forgejo_sdk.ListTrackedTimesOptions) ([]*forgejo_sdk.TrackedTime, error) {
	times, resp, err := a.client.ListIssueTrackedTimes(owner, repo, index, opt)
	return times, statusErr(resp, err)
}

func (a *Adapter) ListRepoTrackedTimes(_ context.Context, owner, repo string, opt forgejo_sdk.ListTrackedTimesOptions) ([]*forgejo_sdk.TrackedTime, error) {
	times, resp, err := a.client.ListRepoTrackedTimes(owner, repo, opt)
	return times, statusErr(resp, err)
}

func (a *Adapter) ListMyTrackedTimes(context.Context) ([]*forgejo_sdk.TrackedTime, error) {
	times, resp, err := a.client.GetMyTrackedTimes()
	return times, statusErr(resp, err)
}

func (a *Adapter) AddIssueTime(_ context.Context, owner, repo string, index int64, opt forgejo_sdk.AddTimeOption) (*forgejo_sdk.TrackedTime, error) {
	entry, resp, err := a.client.AddTime(owner, repo, index, opt)
	return entry, statusErr(resp, err)
}

func (a *Adapter) ResetIssueTime(_ context.Context, owner, repo string, index int64) error {
	resp, err := a.client.ResetIssueTime(owner, repo, index)
	return statusErr(resp, err)
}

func (a *Adapter) DeleteIssueTimeEntry(_ context.Context, owner, repo string, index, timeID int64) error {
	resp, err := a.client.DeleteTime(owner, repo, index, timeID)
	return statusErr(resp, err)
}

func (a *Adapter) StartIssueStopwatch(_ context.Context, owner, repo string, index int64) error {
	resp, err := a.client.StartIssueStopWatch(owner, repo, index)
	return statusErr(resp, err)
}

func (a *Adapter) StopIssueStopwatch(_ context.Context, owner, repo string, index int64) error {
	resp, err := a.client.StopIssueStopWatch(owner, repo, index)
	return statusErr(resp, err)
}

func (a *Adapter) CancelIssueStopwatch(_ context.Context, owner, repo string, index int64) error {
	resp, err := a.client.DeleteIssueStopwatch(owner, repo, index)
	return statusErr(resp, err)
}

func (a *Adapter) ListMyStopwatches(context.Context) ([]*forgejo_sdk.StopWatch, error) {
	watches, resp, err := a.client.GetMyStopwatches()
	return watches, statusErr(resp, err)
}
