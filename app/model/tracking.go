// SPDX-License-Identifier: GPL-3.0-or-later

package model

import "time"

// TrackedTime is one logged time entry, as the time-tracking tools return it.
type TrackedTime struct {
	ID       int64     `json:"id"`
	Created  time.Time `json:"created"`
	Time     int64     `json:"time"`
	UserID   int64     `json:"user_id"`
	UserName string    `json:"user_name"`
	IssueID  int64     `json:"issue_id"`
	Issue    *Issue    `json:"issue"`
}

// StopWatch is a running stopwatch of the authenticated user.
type StopWatch struct {
	Created       time.Time `json:"created"`
	Seconds       int64     `json:"seconds"`
	Duration      string    `json:"duration"`
	IssueIndex    int64     `json:"issue_index"`
	IssueTitle    string    `json:"issue_title"`
	RepoOwnerName string    `json:"repo_owner_name"`
	RepoName      string    `json:"repo_name"`
}
