// SPDX-License-Identifier: GPL-3.0-or-later

package model

// TrackedTimePage is the response of list_issue_tracked_times and
// list_repo_tracked_times: one page of tracked times with the continuation
// signal of docs/design/output-bounding.md.
type TrackedTimePage struct {
	Times   []*TrackedTime `json:"times"`
	Page    int            `json:"page"`
	Limit   int            `json:"limit"`
	Count   int            `json:"count"`
	HasNext bool           `json:"has_next"`
}
