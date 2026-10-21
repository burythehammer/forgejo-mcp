// SPDX-License-Identifier: GPL-3.0-or-later

package model

import "time"

// NotificationThread is one notification of the authenticated user.
type NotificationThread struct {
	ID         int64                `json:"id"`
	Repository *Repository          `json:"repository"`
	Subject    *NotificationSubject `json:"subject"`
	Unread     bool                 `json:"unread"`
	Pinned     bool                 `json:"pinned"`
	UpdatedAt  time.Time            `json:"updated_at"`
	URL        string               `json:"url"`
}

// NotificationSubject is the issue, pull request or commit a notification is about.
type NotificationSubject struct {
	Title                string `json:"title"`
	URL                  string `json:"url"`
	HTMLURL              string `json:"html_url"`
	LatestCommentURL     string `json:"latest_comment_url"`
	LatestCommentHTMLURL string `json:"latest_comment_html_url"`
	Type                 string `json:"type"`
	State                string `json:"state"`
}
