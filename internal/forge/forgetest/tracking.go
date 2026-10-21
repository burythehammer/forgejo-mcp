// SPDX-License-Identifier: GPL-3.0-or-later

package forgetest

import (
	"testing"
	"time"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// RunTracking runs the time-tracking contract against the implementation that
// newHarness builds, once per case.
func RunTracking(t *testing.T, newHarness func(t *testing.T) Harness) {
	cases := []struct {
		name string
		run  func(t *testing.T, h Harness)
	}{
		{"added time reads back on the issue", func(t *testing.T, h Harness) {
			h.Seed.AddTrackableIssue(Owner, Repo, 1, "first")
			entry, err := h.Port.AddIssueTime(ctx, Owner, Repo, 1, forgejo_sdk.AddTimeOption{Time: 90})
			must(t, err)
			if entry.Time != 90 || entry.ID == 0 {
				t.Fatalf("got %+v", entry)
			}
			got, err := h.Port.ListIssueTrackedTimes(ctx, Owner, Repo, 1, forgejo_sdk.ListTrackedTimesOptions{})
			must(t, err)
			if len(got) != 1 || got[0].ID != entry.ID || got[0].Time != 90 {
				t.Fatalf("got %+v", got)
			}
		}},
		{"adding no time is rejected", func(t *testing.T, h Harness) {
			h.Seed.AddTrackableIssue(Owner, Repo, 1, "first")
			if _, err := h.Port.AddIssueTime(ctx, Owner, Repo, 1, forgejo_sdk.AddTimeOption{}); err == nil {
				t.Fatal("want an error")
			}
		}},
		{"unknown issue is not found", func(t *testing.T, h Harness) {
			h.Seed.AddTrackableIssue(Owner, Repo, 1, "first")
			_, err := h.Port.ListIssueTrackedTimes(ctx, Owner, Repo, 9, forgejo_sdk.ListTrackedTimesOptions{})
			wantNotFound(t, err)
			_, err = h.Port.AddIssueTime(ctx, Owner, Repo, 9, forgejo_sdk.AddTimeOption{Time: 5})
			wantNotFound(t, err)
			wantNotFound(t, h.Port.ResetIssueTime(ctx, Owner, Repo, 9))
			wantNotFound(t, h.Port.DeleteIssueTimeEntry(ctx, Owner, Repo, 1, 123))
			wantNotFound(t, h.Port.StartIssueStopwatch(ctx, Owner, Repo, 9))
		}},
		{"times filter by window and paginate", func(t *testing.T, h Harness) {
			h.Seed.AddTrackableIssue(Owner, Repo, 1, "first")
			base := time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)
			for i := range 3 {
				_, err := h.Port.AddIssueTime(ctx, Owner, Repo, 1, forgejo_sdk.AddTimeOption{Time: int64(10 * (i + 1)), Created: base.AddDate(0, 0, i)})
				must(t, err)
			}
			since, err := h.Port.ListIssueTrackedTimes(ctx, Owner, Repo, 1, forgejo_sdk.ListTrackedTimesOptions{Since: base.Add(time.Hour)})
			must(t, err)
			if len(since) != 2 {
				t.Fatalf("since: %+v", since)
			}
			before, err := h.Port.ListIssueTrackedTimes(ctx, Owner, Repo, 1, forgejo_sdk.ListTrackedTimesOptions{Before: base.Add(time.Hour)})
			must(t, err)
			if len(before) != 1 || before[0].Time != 10 {
				t.Fatalf("before: %+v", before)
			}
			second, err := h.Port.ListIssueTrackedTimes(ctx, Owner, Repo, 1, forgejo_sdk.ListTrackedTimesOptions{ListOptions: forgejo_sdk.ListOptions{Page: 2, PageSize: 2}})
			must(t, err)
			if len(second) != 1 || second[0].Time != 30 {
				t.Fatalf("page 2: %+v", second)
			}
		}},
		{"repository times span issues and filter by user", func(t *testing.T, h Harness) {
			h.Seed.AddTrackableIssue(Owner, Repo, 1, "first")
			h.Seed.AddTrackableIssue(Owner, Repo, 2, "second")
			_, err := h.Port.AddIssueTime(ctx, Owner, Repo, 1, forgejo_sdk.AddTimeOption{Time: 10})
			must(t, err)
			_, err = h.Port.AddIssueTime(ctx, Owner, Repo, 2, forgejo_sdk.AddTimeOption{Time: 20, User: "someone-else"})
			must(t, err)
			all, err := h.Port.ListRepoTrackedTimes(ctx, Owner, Repo, forgejo_sdk.ListTrackedTimesOptions{})
			must(t, err)
			if len(all) != 2 {
				t.Fatalf("all: %+v", all)
			}
			one, err := h.Port.ListRepoTrackedTimes(ctx, Owner, Repo, forgejo_sdk.ListTrackedTimesOptions{User: "someone-else"})
			must(t, err)
			if len(one) != 1 || one[0].Time != 20 {
				t.Fatalf("user filter: %+v", one)
			}
			_, err = h.Port.ListRepoTrackedTimes(ctx, "nobody", "nothing", forgejo_sdk.ListTrackedTimesOptions{})
			wantNotFound(t, err)
		}},
		{"my times are the current user's across repositories", func(t *testing.T, h Harness) {
			h.Seed.AddTrackableIssue(Owner, Repo, 1, "first")
			h.Seed.AddTrackableIssue(Owner, "other", 1, "elsewhere")
			_, err := h.Port.AddIssueTime(ctx, Owner, Repo, 1, forgejo_sdk.AddTimeOption{Time: 10})
			must(t, err)
			_, err = h.Port.AddIssueTime(ctx, Owner, "other", 1, forgejo_sdk.AddTimeOption{Time: 20})
			must(t, err)
			_, err = h.Port.AddIssueTime(ctx, Owner, Repo, 1, forgejo_sdk.AddTimeOption{Time: 30, User: "someone-else"})
			must(t, err)
			mine, err := h.Port.ListMyTrackedTimes(ctx)
			must(t, err)
			var total int64
			for _, e := range mine {
				total += e.Time
			}
			if len(mine) != 2 || total != 30 {
				t.Fatalf("got %+v", mine)
			}
		}},
		{"delete removes one entry and reset removes all", func(t *testing.T, h Harness) {
			h.Seed.AddTrackableIssue(Owner, Repo, 1, "first")
			a, err := h.Port.AddIssueTime(ctx, Owner, Repo, 1, forgejo_sdk.AddTimeOption{Time: 10})
			must(t, err)
			_, err = h.Port.AddIssueTime(ctx, Owner, Repo, 1, forgejo_sdk.AddTimeOption{Time: 20})
			must(t, err)
			must(t, h.Port.DeleteIssueTimeEntry(ctx, Owner, Repo, 1, a.ID))
			left, err := h.Port.ListIssueTrackedTimes(ctx, Owner, Repo, 1, forgejo_sdk.ListTrackedTimesOptions{})
			must(t, err)
			if len(left) != 1 || left[0].Time != 20 {
				t.Fatalf("got %+v", left)
			}
			wantNotFound(t, h.Port.DeleteIssueTimeEntry(ctx, Owner, Repo, 1, a.ID))
			must(t, h.Port.ResetIssueTime(ctx, Owner, Repo, 1))
			left, err = h.Port.ListIssueTrackedTimes(ctx, Owner, Repo, 1, forgejo_sdk.ListTrackedTimesOptions{})
			must(t, err)
			if len(left) != 0 {
				t.Fatalf("after reset: %+v", left)
			}
		}},
		{"a started stopwatch lists, and stop records time", func(t *testing.T, h Harness) {
			h.Seed.AddTrackableIssue(Owner, Repo, 1, "first")
			must(t, h.Port.StartIssueStopwatch(ctx, Owner, Repo, 1))
			watches, err := h.Port.ListMyStopwatches(ctx)
			must(t, err)
			if len(watches) != 1 || watches[0].IssueIndex != 1 || watches[0].IssueTitle != "first" || watches[0].RepoName != Repo || watches[0].RepoOwnerName != Owner {
				t.Fatalf("got %+v", watches)
			}
			if err := h.Port.StartIssueStopwatch(ctx, Owner, Repo, 1); forge.StatusCode(err) != 409 {
				t.Fatalf("starting twice: %v", err)
			}
			must(t, h.Port.StopIssueStopwatch(ctx, Owner, Repo, 1))
			watches, err = h.Port.ListMyStopwatches(ctx)
			must(t, err)
			if len(watches) != 0 {
				t.Fatalf("after stop: %+v", watches)
			}
			times, err := h.Port.ListIssueTrackedTimes(ctx, Owner, Repo, 1, forgejo_sdk.ListTrackedTimesOptions{})
			must(t, err)
			if len(times) != 1 {
				t.Fatalf("stop must record an entry, got %+v", times)
			}
			if err := h.Port.StopIssueStopwatch(ctx, Owner, Repo, 1); forge.StatusCode(err) != 409 {
				t.Fatalf("stopping twice: %v", err)
			}
		}},
		{"a cancelled stopwatch records nothing", func(t *testing.T, h Harness) {
			h.Seed.AddTrackableIssue(Owner, Repo, 1, "first")
			must(t, h.Port.StartIssueStopwatch(ctx, Owner, Repo, 1))
			must(t, h.Port.CancelIssueStopwatch(ctx, Owner, Repo, 1))
			times, err := h.Port.ListIssueTrackedTimes(ctx, Owner, Repo, 1, forgejo_sdk.ListTrackedTimesOptions{})
			must(t, err)
			if len(times) != 0 {
				t.Fatalf("got %+v", times)
			}
			if err := h.Port.CancelIssueStopwatch(ctx, Owner, Repo, 1); forge.StatusCode(err) != 409 {
				t.Fatalf("cancelling twice: %v", err)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.run(t, newHarness(t))
		})
	}
}
