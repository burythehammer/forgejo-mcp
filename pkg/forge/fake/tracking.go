// SPDX-License-Identifier: GPL-3.0-or-later

package fake

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var _ forge.Tracking = (*Forgejo)(nil)

type trackingState struct {
	mu     sync.Mutex
	issues map[string]*trackedIssue // key "owner/repo#index"
	nextID int64
}

type trackedIssue struct {
	owner, repo string
	index       int64
	title       string
	times       []*forgejo_sdk.TrackedTime
	// stopwatch is when the current user's stopwatch started; zero when none.
	stopwatch time.Time
}

func (s *trackingState) issue(owner, repo string, index int64) (*trackedIssue, error) {
	i, ok := s.issues[fmt.Sprintf("%s/%s#%d", owner, repo, index)]
	if !ok {
		return nil, notFound("issue")
	}
	return i, nil
}

// AddTrackableIssue makes issue #index of owner/repo exist for time tracking
// (and the repository, as AddRepo would). Everything under an unknown issue is
// 404, as on Forgejo. The current user is Poster.
func (f *Forgejo) AddTrackableIssue(owner, repo string, index int64, title string) {
	f.AddRepo(owner, repo)
	f.tracking.mu.Lock()
	defer f.tracking.mu.Unlock()
	if f.tracking.issues == nil {
		f.tracking.issues = map[string]*trackedIssue{}
	}
	f.tracking.issues[fmt.Sprintf("%s/%s#%d", owner, repo, index)] = &trackedIssue{owner: owner, repo: repo, index: index, title: title}
}

func (f *Forgejo) addEntry(i *trackedIssue, seconds int64, created time.Time, userName string) *forgejo_sdk.TrackedTime {
	f.tracking.nextID++
	if created.IsZero() {
		created = f.Now()
	}
	if userName == "" {
		userName = Poster
	}
	t := &forgejo_sdk.TrackedTime{
		ID: f.tracking.nextID, Created: created, Time: seconds, UserName: userName, Issue: &forgejo_sdk.Issue{Index: i.index, Title: i.title},
	}
	i.times = append(i.times, t)
	return t
}

func inWindow(t *forgejo_sdk.TrackedTime, opt forgejo_sdk.ListTrackedTimesOptions) bool {
	if !opt.Since.IsZero() && t.Created.Before(opt.Since) {
		return false
	}
	if !opt.Before.IsZero() && t.Created.After(opt.Before) {
		return false
	}
	return true
}

func pageTimes(all []*forgejo_sdk.TrackedTime, opt forgejo_sdk.ListOptions) []*forgejo_sdk.TrackedTime {
	sort.SliceStable(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	lo, hi := page(opt, len(all))
	out := make([]*forgejo_sdk.TrackedTime, 0, hi-lo)
	for _, t := range all[lo:hi] {
		cp := *t
		out = append(out, &cp)
	}
	return out
}

func (f *Forgejo) ListIssueTrackedTimes(_ context.Context, owner, repo string, index int64, opt forgejo_sdk.ListTrackedTimesOptions) ([]*forgejo_sdk.TrackedTime, error) {
	f.tracking.mu.Lock()
	defer f.tracking.mu.Unlock()
	i, err := f.tracking.issue(owner, repo, index)
	if err != nil {
		return nil, err
	}
	var all []*forgejo_sdk.TrackedTime
	for _, t := range i.times {
		if inWindow(t, opt) {
			all = append(all, t)
		}
	}
	return pageTimes(all, opt.ListOptions), nil
}

func (f *Forgejo) ListRepoTrackedTimes(_ context.Context, owner, repo string, opt forgejo_sdk.ListTrackedTimesOptions) ([]*forgejo_sdk.TrackedTime, error) {
	if !f.repoExists(owner, repo) {
		return nil, notFound("repository")
	}
	f.tracking.mu.Lock()
	defer f.tracking.mu.Unlock()
	var all []*forgejo_sdk.TrackedTime
	for _, i := range f.tracking.issues {
		if i.owner != owner || i.repo != repo {
			continue
		}
		for _, t := range i.times {
			if inWindow(t, opt) && (opt.User == "" || t.UserName == opt.User) {
				all = append(all, t)
			}
		}
	}
	return pageTimes(all, opt.ListOptions), nil
}

func (f *Forgejo) ListMyTrackedTimes(context.Context) ([]*forgejo_sdk.TrackedTime, error) {
	f.tracking.mu.Lock()
	defer f.tracking.mu.Unlock()
	var all []*forgejo_sdk.TrackedTime
	for _, i := range f.tracking.issues {
		for _, t := range i.times {
			if t.UserName == Poster {
				all = append(all, t)
			}
		}
	}
	return pageTimes(all, forgejo_sdk.ListOptions{Page: -1}), nil
}

func (f *Forgejo) AddIssueTime(_ context.Context, owner, repo string, index int64, opt forgejo_sdk.AddTimeOption) (*forgejo_sdk.TrackedTime, error) {
	if err := opt.Validate(); err != nil {
		return nil, err
	}
	f.tracking.mu.Lock()
	defer f.tracking.mu.Unlock()
	i, err := f.tracking.issue(owner, repo, index)
	if err != nil {
		return nil, err
	}
	if opt.Time < 0 {
		return nil, statusErr(http.StatusBadRequest, "time must be positive")
	}
	t := *f.addEntry(i, opt.Time, opt.Created, opt.User)
	return &t, nil
}

func (f *Forgejo) ResetIssueTime(_ context.Context, owner, repo string, index int64) error {
	f.tracking.mu.Lock()
	defer f.tracking.mu.Unlock()
	i, err := f.tracking.issue(owner, repo, index)
	if err != nil {
		return err
	}
	i.times = nil
	return nil
}

func (f *Forgejo) DeleteIssueTimeEntry(_ context.Context, owner, repo string, index, timeID int64) error {
	f.tracking.mu.Lock()
	defer f.tracking.mu.Unlock()
	i, err := f.tracking.issue(owner, repo, index)
	if err != nil {
		return err
	}
	for n, t := range i.times {
		if t.ID == timeID {
			i.times = append(i.times[:n], i.times[n+1:]...)
			return nil
		}
	}
	return notFound("time entry")
}

func (f *Forgejo) StartIssueStopwatch(_ context.Context, owner, repo string, index int64) error {
	f.tracking.mu.Lock()
	defer f.tracking.mu.Unlock()
	i, err := f.tracking.issue(owner, repo, index)
	if err != nil {
		return err
	}
	if !i.stopwatch.IsZero() {
		return statusErr(http.StatusConflict, "cannot start a stopwatch again if it already exists")
	}
	i.stopwatch = f.Now()
	return nil
}

func (f *Forgejo) StopIssueStopwatch(_ context.Context, owner, repo string, index int64) error {
	f.tracking.mu.Lock()
	defer f.tracking.mu.Unlock()
	i, err := f.tracking.issue(owner, repo, index)
	if err != nil {
		return err
	}
	if i.stopwatch.IsZero() {
		return statusErr(http.StatusConflict, "cannot stop a non-existent stopwatch")
	}
	seconds := int64(f.Now().Sub(i.stopwatch).Seconds())
	i.stopwatch = time.Time{}
	f.addEntry(i, max(seconds, 0), time.Time{}, "")
	return nil
}

func (f *Forgejo) CancelIssueStopwatch(_ context.Context, owner, repo string, index int64) error {
	f.tracking.mu.Lock()
	defer f.tracking.mu.Unlock()
	i, err := f.tracking.issue(owner, repo, index)
	if err != nil {
		return err
	}
	if i.stopwatch.IsZero() {
		return statusErr(http.StatusConflict, "cannot cancel a non-existent stopwatch")
	}
	i.stopwatch = time.Time{}
	return nil
}

func (f *Forgejo) ListMyStopwatches(context.Context) ([]*forgejo_sdk.StopWatch, error) {
	f.tracking.mu.Lock()
	defer f.tracking.mu.Unlock()
	now := f.Now()
	out := []*forgejo_sdk.StopWatch{}
	for _, i := range f.tracking.issues {
		if i.stopwatch.IsZero() {
			continue
		}
		secs := int64(now.Sub(i.stopwatch).Seconds())
		out = append(out, &forgejo_sdk.StopWatch{
			Created: i.stopwatch, Seconds: secs, Duration: (time.Duration(secs) * time.Second).String(),
			IssueIndex: i.index, IssueTitle: i.title, RepoOwnerName: i.owner, RepoName: i.repo,
		})
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].RepoName != out[b].RepoName {
			return out[a].RepoName < out[b].RepoName
		}
		return out[a].IssueIndex < out[b].IssueIndex
	})
	return out, nil
}
