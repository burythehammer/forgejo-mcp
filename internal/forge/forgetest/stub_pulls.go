// SPDX-License-Identifier: GPL-3.0-or-later

package forgetest

import (
	"net/http"
	"strconv"
	"strings"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

const pullsBase = "/api/v1/repos/{owner}/{repo}/pulls"

// mountPulls serves the pull-request endpoints from the stub's Fake.
func (s *Stub) mountPulls(mux *http.ServeMux) {
	f := s.Fake

	mux.HandleFunc("GET "+pullsBase, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		ms, _ := strconv.ParseInt(q.Get("milestone"), 10, 64)
		out, err := f.ListPullRequests(r.Context(), r.PathValue("owner"), r.PathValue("repo"), forgejo_sdk.ListPullRequestsOptions{
			ListOptions: listOptions(r),
			State:       forgejo_sdk.StateType(q.Get("state")),
			Sort:        q.Get("sort"),
			Milestone:   ms,
		})
		reply(w, out, err)
	})
	mux.HandleFunc("POST "+pullsBase, func(w http.ResponseWriter, r *http.Request) {
		var opt forgejo_sdk.CreatePullRequestOption
		if !decode(w, r, &opt) {
			return
		}
		pr, err := f.CreatePullRequest(r.Context(), r.PathValue("owner"), r.PathValue("repo"), opt)
		replyStatus(w, http.StatusCreated, pr, err)
	})
	// {index} also carries the ".diff" suffix of the diff download.
	mux.HandleFunc("GET "+pullsBase+"/{index}", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.PathValue("index"), ".diff") {
			diff, err := f.GetPullRequestDiff(r.Context(), r.PathValue("owner"), r.PathValue("repo"), pathIndex(r, "index"))
			if err != nil {
				writeErr(w, err)
				return
			}
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write(diff)
			return
		}
		pr, err := f.GetPullRequest(r.Context(), r.PathValue("owner"), r.PathValue("repo"), pathIndex(r, "index"))
		reply(w, pr, err)
	})
	mux.HandleFunc("PATCH "+pullsBase+"/{index}", func(w http.ResponseWriter, r *http.Request) {
		var opt forgejo_sdk.EditPullRequestOption
		if !decode(w, r, &opt) {
			return
		}
		pr, err := f.EditPullRequest(r.Context(), r.PathValue("owner"), r.PathValue("repo"), pathIndex(r, "index"), opt)
		reply(w, pr, err)
	})
	mux.HandleFunc("POST "+pullsBase+"/{index}/merge", func(w http.ResponseWriter, r *http.Request) {
		var opt forgejo_sdk.MergePullRequestOption
		if !decode(w, r, &opt) {
			return
		}
		res, err := f.MergePullRequest(r.Context(), r.PathValue("owner"), r.PathValue("repo"), pathIndex(r, "index"), opt)
		if err != nil {
			writeErr(w, err)
			return
		}
		switch {
		case res.StatusCode < http.StatusMultipleChoices:
			w.WriteHeader(res.StatusCode)
		case res.Message != "":
			writeJSON(w, res.StatusCode, map[string]string{"message": res.Message})
		default:
			// A declined answer whose body is not JSON, as a proxy might send.
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(res.StatusCode)
			_, _ = w.Write([]byte("declined\n"))
		}
	})
	mux.HandleFunc("GET "+pullsBase+"/{index}/files", func(w http.ResponseWriter, r *http.Request) {
		out, err := f.ListPullRequestFiles(r.Context(), r.PathValue("owner"), r.PathValue("repo"), pathIndex(r, "index"),
			forgejo_sdk.ListPullRequestFilesOptions{ListOptions: listOptions(r)})
		reply(w, out, err)
	})

	mux.HandleFunc("GET "+pullsBase+"/{index}/reviews", func(w http.ResponseWriter, r *http.Request) {
		out, info, err := f.ListPullReviews(r.Context(), r.PathValue("owner"), r.PathValue("repo"), pathIndex(r, "index"),
			forgejo_sdk.ListPullReviewsOptions{ListOptions: listOptions(r)})
		if err == nil && info.TotalKnown {
			w.Header().Set("X-Total-Count", strconv.Itoa(info.TotalCount))
		}
		if err == nil && info.HasNext {
			page := max(listOptions(r).Page, 1)
			q := r.URL.Query()
			q.Set("page", strconv.Itoa(page+1))
			w.Header().Set("Link", `<`+r.URL.Path+`?`+q.Encode()+`>; rel="next"`)
		}
		reply(w, out, err)
	})
	mux.HandleFunc("POST "+pullsBase+"/{index}/reviews", func(w http.ResponseWriter, r *http.Request) {
		var opt forgejo_sdk.CreatePullReviewOptions
		if !decode(w, r, &opt) {
			return
		}
		out, err := f.CreatePullReview(r.Context(), r.PathValue("owner"), r.PathValue("repo"), pathIndex(r, "index"), opt)
		reply(w, out, err)
	})
	mux.HandleFunc("GET "+pullsBase+"/{index}/reviews/{id}", func(w http.ResponseWriter, r *http.Request) {
		out, err := f.GetPullReview(r.Context(), r.PathValue("owner"), r.PathValue("repo"), pathIndex(r, "index"), pathIndex(r, "id"))
		reply(w, out, err)
	})
	mux.HandleFunc("POST "+pullsBase+"/{index}/reviews/{id}", func(w http.ResponseWriter, r *http.Request) {
		var opt forgejo_sdk.SubmitPullReviewOptions
		if !decode(w, r, &opt) {
			return
		}
		out, err := f.SubmitPullReview(r.Context(), r.PathValue("owner"), r.PathValue("repo"), pathIndex(r, "index"), pathIndex(r, "id"), opt)
		reply(w, out, err)
	})
	mux.HandleFunc("DELETE "+pullsBase+"/{index}/reviews/{id}", func(w http.ResponseWriter, r *http.Request) {
		replyEmpty(w, f.DeletePullReview(r.Context(), r.PathValue("owner"), r.PathValue("repo"), pathIndex(r, "index"), pathIndex(r, "id")))
	})
	mux.HandleFunc("GET "+pullsBase+"/{index}/reviews/{id}/comments", func(w http.ResponseWriter, r *http.Request) {
		out, err := f.ListPullReviewComments(r.Context(), r.PathValue("owner"), r.PathValue("repo"), pathIndex(r, "index"), pathIndex(r, "id"))
		reply(w, out, err)
	})
	mux.HandleFunc("POST "+pullsBase+"/{index}/reviews/{id}/dismissals", func(w http.ResponseWriter, r *http.Request) {
		var opt forgejo_sdk.DismissPullReviewOptions
		if !decode(w, r, &opt) {
			return
		}
		replyEmpty(w, f.DismissPullReview(r.Context(), r.PathValue("owner"), r.PathValue("repo"), pathIndex(r, "index"), pathIndex(r, "id"), opt))
	})
	mux.HandleFunc("POST "+pullsBase+"/{index}/requested_reviewers", func(w http.ResponseWriter, r *http.Request) {
		var opt forgejo_sdk.PullReviewRequestOptions
		if !decode(w, r, &opt) {
			return
		}
		replyStatusEmpty(w, http.StatusCreated, f.CreateReviewRequests(r.Context(), r.PathValue("owner"), r.PathValue("repo"), pathIndex(r, "index"), opt))
	})
	mux.HandleFunc("DELETE "+pullsBase+"/{index}/requested_reviewers", func(w http.ResponseWriter, r *http.Request) {
		var opt forgejo_sdk.PullReviewRequestOptions
		if !decode(w, r, &opt) {
			return
		}
		replyEmpty(w, f.DeleteReviewRequests(r.Context(), r.PathValue("owner"), r.PathValue("repo"), pathIndex(r, "index"), opt))
	})
}

func reply(w http.ResponseWriter, v any, err error) { replyStatus(w, http.StatusOK, v, err) }

func replyStatus(w http.ResponseWriter, status int, v any, err error) {
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, status, v)
}

func replyEmpty(w http.ResponseWriter, err error) { replyStatusEmpty(w, http.StatusNoContent, err) }

func replyStatusEmpty(w http.ResponseWriter, status int, err error) {
	if err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(status)
}
