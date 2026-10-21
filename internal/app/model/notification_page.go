// SPDX-License-Identifier: GPL-3.0-or-later

package model

// NotificationPage is the response of list_repo_notifications: one page of a
// repository's notification threads.
type NotificationPage struct {
	Notifications []*NotificationThread `json:"notifications"`
	PageMeta
}
