// SPDX-License-Identifier: GPL-3.0-or-later

package model

// TrackedTimePage is the response of list_issue_tracked_times and
// list_repo_tracked_times: one page of tracked times with the continuation
// signal of docs/design/output-bounding.md.
type TrackedTimePage struct {
	Times []*TrackedTime `json:"times"`
	PageMeta
}

// StopWatchPage is the response of list_my_stopwatches: one page of the
// running stopwatches with the continuation signal of docs/design/output-bounding.md.
type StopWatchPage struct {
	Stopwatches []*StopWatch `json:"stopwatches"`
	PageMeta
}
