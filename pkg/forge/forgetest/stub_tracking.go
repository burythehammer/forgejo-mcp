// SPDX-License-Identifier: GPL-3.0-or-later

package forgetest

import (
	"net/http"
	"net/url"
	"time"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

const issueBase = "/api/v1/repos/{owner}/{repo}/issues/{index}"

func trackedTimesOptions(r *http.Request) forgejo_sdk.ListTrackedTimesOptions {
	q := r.URL.Query()
	opt := forgejo_sdk.ListTrackedTimesOptions{ListOptions: listOptions(r), User: q.Get("user")}
	opt.Since = parseTime(q, "since")
	opt.Before = parseTime(q, "before")
	return opt
}

func parseTime(q url.Values, key string) time.Time {
	t, _ := time.Parse(time.RFC3339, q.Get(key))
	return t
}

// mountTracking serves the time-tracking and stopwatch endpoints from the
// stub's Fake.
func (s *Stub) mountTracking(mux *http.ServeMux) {
	f := s.Fake
	repo := func(r *http.Request) (string, string, int64) {
		return r.PathValue("owner"), r.PathValue("repo"), pathIndex(r, "index")
	}

	mux.HandleFunc("GET "+issueBase+"/times", func(w http.ResponseWriter, r *http.Request) {
		o, n, i := repo(r)
		out, err := f.ListIssueTrackedTimes(r.Context(), o, n, i, trackedTimesOptions(r))
		reply(w, out, err)
	})
	mux.HandleFunc("POST "+issueBase+"/times", func(w http.ResponseWriter, r *http.Request) {
		var opt forgejo_sdk.AddTimeOption
		if !decode(w, r, &opt) {
			return
		}
		o, n, i := repo(r)
		out, err := f.AddIssueTime(r.Context(), o, n, i, opt)
		reply(w, out, err)
	})
	mux.HandleFunc("DELETE "+issueBase+"/times", func(w http.ResponseWriter, r *http.Request) {
		o, n, i := repo(r)
		replyEmpty(w, f.ResetIssueTime(r.Context(), o, n, i))
	})
	mux.HandleFunc("DELETE "+issueBase+"/times/{id}", func(w http.ResponseWriter, r *http.Request) {
		o, n, i := repo(r)
		replyEmpty(w, f.DeleteIssueTimeEntry(r.Context(), o, n, i, pathIndex(r, "id")))
	})
	mux.HandleFunc("GET /api/v1/repos/{owner}/{repo}/times", func(w http.ResponseWriter, r *http.Request) {
		out, err := f.ListRepoTrackedTimes(r.Context(), r.PathValue("owner"), r.PathValue("repo"), trackedTimesOptions(r))
		reply(w, out, err)
	})
	mux.HandleFunc("GET /api/v1/user/times", func(w http.ResponseWriter, r *http.Request) {
		out, err := f.ListMyTrackedTimes(r.Context())
		reply(w, out, err)
	})

	mux.HandleFunc("POST "+issueBase+"/stopwatch/start", func(w http.ResponseWriter, r *http.Request) {
		o, n, i := repo(r)
		replyStatusEmpty(w, http.StatusCreated, f.StartIssueStopwatch(r.Context(), o, n, i))
	})
	mux.HandleFunc("POST "+issueBase+"/stopwatch/stop", func(w http.ResponseWriter, r *http.Request) {
		o, n, i := repo(r)
		replyStatusEmpty(w, http.StatusCreated, f.StopIssueStopwatch(r.Context(), o, n, i))
	})
	mux.HandleFunc("DELETE "+issueBase+"/stopwatch/delete", func(w http.ResponseWriter, r *http.Request) {
		o, n, i := repo(r)
		replyEmpty(w, f.CancelIssueStopwatch(r.Context(), o, n, i))
	})
	mux.HandleFunc("GET /api/v1/user/stopwatches", func(w http.ResponseWriter, r *http.Request) {
		out, err := f.ListMyStopwatches(r.Context())
		reply(w, out, err)
	})
}
