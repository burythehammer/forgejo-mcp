// SPDX-License-Identifier: GPL-3.0-or-later

package forgetest

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

const issuesBase = "/api/v1/repos/{owner}/{repo}/issues"

// replyPaged is reply for a list: it also sends the Link and X-Total-Count
// headers Forgejo sends, which is how the adapter learns about further pages.
func replyPaged(w http.ResponseWriter, r *http.Request, v any, info forge.PageInfo, err error) {
	if err != nil {
		writeErr(w, err)
		return
	}
	if info.TotalKnown {
		w.Header().Set("X-Total-Count", strconv.Itoa(info.TotalCount))
	}
	if info.HasNext {
		page := listOptions(r).Page
		if page < 1 {
			page = 1
		}
		q := r.URL.Query()
		q.Set("page", strconv.Itoa(page+1))
		w.Header().Set("Link", `<`+r.URL.Path+`?`+q.Encode()+`>; rel="next"`)
	}
	writeJSON(w, http.StatusOK, v)
}

func issueListOption(r *http.Request) forgejo_sdk.ListIssueOption {
	q := r.URL.Query()
	opt := forgejo_sdk.ListIssueOption{
		ListOptions: listOptions(r),
		State:       forgejo_sdk.StateType(q.Get("state")),
		Type:        forgejo_sdk.IssueType(q.Get("type")),
		KeyWord:     q.Get("q"),
		CreatedBy:   q.Get("created_by"),
		AssignedBy:  q.Get("assigned_by"),
		Owner:       q.Get("owner"),
	}
	if v := q.Get("labels"); v != "" {
		opt.Labels = strings.Split(v, ",")
	}
	if v := q.Get("milestones"); v != "" {
		opt.Milestones = strings.Split(v, ",")
	}
	return opt
}

func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// mountIssues serves the issue endpoints from the stub's Fake.
//
// GET and DELETE of {index}/comments, {index}/labels and comments/{id} overlap
// in the mux, so those two methods dispatch on the segments in one handler.
func (s *Stub) mountIssues(mux *http.ServeMux) {
	f := s.Fake

	mux.HandleFunc("GET /api/v1/repos/issues/search", func(w http.ResponseWriter, r *http.Request) {
		out, info, err := f.SearchIssues(r.Context(), issueListOption(r))
		replyPaged(w, r, out, info, err)
	})
	mux.HandleFunc("GET "+issuesBase, func(w http.ResponseWriter, r *http.Request) {
		out, info, err := f.ListRepoIssues(r.Context(), r.PathValue("owner"), r.PathValue("repo"),
			forge.ListIssuesOptions{ListIssueOption: issueListOption(r), Sort: r.URL.Query().Get("sort")})
		replyPaged(w, r, out, info, err)
	})
	mux.HandleFunc("POST "+issuesBase, func(w http.ResponseWriter, r *http.Request) {
		var opt forgejo_sdk.CreateIssueOption
		if !decode(w, r, &opt) {
			return
		}
		iss, err := f.CreateIssue(r.Context(), r.PathValue("owner"), r.PathValue("repo"), opt)
		replyStatus(w, http.StatusCreated, iss, err)
	})
	mux.HandleFunc("GET "+issuesBase+"/{index}", func(w http.ResponseWriter, r *http.Request) {
		iss, err := f.GetIssue(r.Context(), r.PathValue("owner"), r.PathValue("repo"), pathIndex(r, "index"))
		reply(w, iss, err)
	})
	mux.HandleFunc("PATCH "+issuesBase+"/{index}", func(w http.ResponseWriter, r *http.Request) {
		var opt forgejo_sdk.EditIssueOption
		if !decode(w, r, &opt) {
			return
		}
		iss, err := f.EditIssue(r.Context(), r.PathValue("owner"), r.PathValue("repo"), pathIndex(r, "index"), opt)
		reply(w, iss, err)
	})

	mux.HandleFunc("GET "+issuesBase+"/{a}/{b}", func(w http.ResponseWriter, r *http.Request) {
		owner, repo := r.PathValue("owner"), r.PathValue("repo")
		switch {
		case r.PathValue("a") == "comments":
			c, err := f.GetIssueComment(r.Context(), owner, repo, pathIndex(r, "b"))
			reply(w, c, err)
		case r.PathValue("b") == "comments":
			q := r.URL.Query()
			out, info, err := f.ListIssueComments(r.Context(), owner, repo, pathIndex(r, "a"), forgejo_sdk.ListIssueCommentOptions{
				ListOptions: listOptions(r), Since: parseTime(q.Get("since")), Before: parseTime(q.Get("before")),
			})
			replyPaged(w, r, out, info, err)
		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("POST "+issuesBase+"/{index}/comments", func(w http.ResponseWriter, r *http.Request) {
		var opt forgejo_sdk.CreateIssueCommentOption
		if !decode(w, r, &opt) {
			return
		}
		c, err := f.CreateIssueComment(r.Context(), r.PathValue("owner"), r.PathValue("repo"), pathIndex(r, "index"), opt.Body)
		replyStatus(w, http.StatusCreated, c, err)
	})
	mux.HandleFunc("PATCH "+issuesBase+"/comments/{id}", func(w http.ResponseWriter, r *http.Request) {
		var opt forgejo_sdk.EditIssueCommentOption
		if !decode(w, r, &opt) {
			return
		}
		c, err := f.EditIssueComment(r.Context(), r.PathValue("owner"), r.PathValue("repo"), pathIndex(r, "id"), opt.Body)
		reply(w, c, err)
	})
	mux.HandleFunc("DELETE "+issuesBase+"/{a}/{b}", func(w http.ResponseWriter, r *http.Request) {
		owner, repo := r.PathValue("owner"), r.PathValue("repo")
		switch {
		case r.PathValue("a") == "comments":
			replyEmpty(w, f.DeleteIssueComment(r.Context(), owner, repo, pathIndex(r, "b")))
		case r.PathValue("b") == "labels":
			replyEmpty(w, f.ClearIssueLabels(r.Context(), owner, repo, pathIndex(r, "a")))
		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("PUT "+issuesBase+"/{index}/labels", func(w http.ResponseWriter, r *http.Request) {
		var opt forgejo_sdk.IssueLabelsOption
		if !decode(w, r, &opt) {
			return
		}
		err := f.ReplaceIssueLabels(r.Context(), r.PathValue("owner"), r.PathValue("repo"), pathIndex(r, "index"), opt.Labels)
		reply(w, []*forgejo_sdk.Label{}, err)
	})

	mux.HandleFunc("GET /api/v1/repos/{owner}/{repo}/milestones", func(w http.ResponseWriter, r *http.Request) {
		out, err := f.ListRepoMilestones(r.Context(), r.PathValue("owner"), r.PathValue("repo"),
			forgejo_sdk.ListMilestoneOption{ListOptions: listOptions(r), State: forgejo_sdk.StateType(r.URL.Query().Get("state"))})
		reply(w, out, err)
	})
}
