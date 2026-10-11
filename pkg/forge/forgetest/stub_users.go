// SPDX-License-Identifier: GPL-3.0-or-later

package forgetest

import (
	"net/http"
	"net/url"
	"time"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// mountUsers serves the user and notification endpoints from the stub's Fake.
func (s *Stub) mountUsers(mux *http.ServeMux) {
	f := s.Fake

	mux.HandleFunc("GET /api/v1/user", func(w http.ResponseWriter, r *http.Request) {
		u, err := f.GetMyUserInfo(r.Context())
		reply(w, u, err)
	})
	mux.HandleFunc("GET /api/v1/users/{user}", func(w http.ResponseWriter, r *http.Request) {
		u, err := f.GetUserInfo(r.Context(), r.PathValue("user"))
		reply(w, u, err)
	})

	mux.HandleFunc("GET /api/v1/notifications", func(w http.ResponseWriter, r *http.Request) {
		out, err := f.ListNotifications(r.Context(), listNotificationOptions(r))
		reply(w, out, err)
	})
	// Marking read answers the threads it changed, as Forgejo 1.16 and later do.
	mux.HandleFunc("PUT /api/v1/notifications", func(w http.ResponseWriter, r *http.Request) {
		replyEmptyList(w, f.ReadNotifications(r.Context(), markNotificationOptions(r)))
	})
	mux.HandleFunc("GET /api/v1/notifications/threads/{id}", func(w http.ResponseWriter, r *http.Request) {
		n, err := f.GetNotification(r.Context(), pathIndex(r, "id"))
		reply(w, n, err)
	})
	mux.HandleFunc("PATCH /api/v1/notifications/threads/{id}", func(w http.ResponseWriter, r *http.Request) {
		n, err := f.ReadNotification(r.Context(), pathIndex(r, "id"))
		reply(w, n, err)
	})
	mux.HandleFunc("GET /api/v1/repos/{owner}/{repo}/notifications", func(w http.ResponseWriter, r *http.Request) {
		out, err := f.ListRepoNotifications(r.Context(), r.PathValue("owner"), r.PathValue("repo"), listNotificationOptions(r))
		reply(w, out, err)
	})
	mux.HandleFunc("PUT /api/v1/repos/{owner}/{repo}/notifications", func(w http.ResponseWriter, r *http.Request) {
		replyEmptyList(w, f.ReadRepoNotifications(r.Context(), r.PathValue("owner"), r.PathValue("repo"), markNotificationOptions(r)))
	})
}

func replyEmptyList(w http.ResponseWriter, err error) {
	reply(w, []*forgejo_sdk.NotificationThread{}, err)
}

func listNotificationOptions(r *http.Request) forgejo_sdk.ListNotificationOptions {
	q := r.URL.Query()
	opt := forgejo_sdk.ListNotificationOptions{ListOptions: listOptions(r), Status: statuses(q)}
	opt.Since, _ = time.Parse(time.RFC3339, q.Get("since"))
	opt.Before, _ = time.Parse(time.RFC3339, q.Get("before"))
	return opt
}

func markNotificationOptions(r *http.Request) forgejo_sdk.MarkNotificationOptions {
	q := r.URL.Query()
	opt := forgejo_sdk.MarkNotificationOptions{Status: statuses(q), ToStatus: forgejo_sdk.NotifyStatus(q.Get("to-status"))}
	opt.LastReadAt, _ = time.Parse(time.RFC3339, q.Get("last_read_at"))
	return opt
}

func statuses(q url.Values) []forgejo_sdk.NotifyStatus {
	var out []forgejo_sdk.NotifyStatus
	for _, s := range q["status-types"] {
		out = append(out, forgejo_sdk.NotifyStatus(s))
	}
	return out
}
