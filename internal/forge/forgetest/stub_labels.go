// SPDX-License-Identifier: GPL-3.0-or-later

package forgetest

import (
	"net/http"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// mountLabels serves the label endpoints and the issue endpoints that add to
// them from the stub's Fake. The GET and DELETE issue routes that overlap
// comments/{id} (dependencies, blocks, labels) live in mountIssues' dispatchers.
func (s *Stub) mountLabels(mux *http.ServeMux) {
	f := s.Fake
	const repoLabels = "/api/v1/repos/{owner}/{repo}/labels"
	const orgLabels = "/api/v1/orgs/{org}/labels"

	mux.HandleFunc("GET "+repoLabels, func(w http.ResponseWriter, r *http.Request) {
		lo := listOptions(r)
		out, info, err := f.ListRepoLabels(r.Context(), r.PathValue("owner"), r.PathValue("repo"), lo.Page, lo.PageSize)
		replyPaged(w, r, out, info, err)
	})
	mux.HandleFunc("POST "+repoLabels, func(w http.ResponseWriter, r *http.Request) {
		var opt forge.LabelCreate
		if !decode(w, r, &opt) {
			return
		}
		l, err := f.CreateRepoLabel(r.Context(), r.PathValue("owner"), r.PathValue("repo"), opt)
		replyStatus(w, http.StatusCreated, l, err)
	})
	mux.HandleFunc("GET "+repoLabels+"/{id}", func(w http.ResponseWriter, r *http.Request) {
		l, err := f.GetRepoLabel(r.Context(), r.PathValue("owner"), r.PathValue("repo"), pathIndex(r, "id"))
		reply(w, l, err)
	})
	mux.HandleFunc("PATCH "+repoLabels+"/{id}", func(w http.ResponseWriter, r *http.Request) {
		var opt forge.LabelPatch
		if !decode(w, r, &opt) {
			return
		}
		l, err := f.EditRepoLabel(r.Context(), r.PathValue("owner"), r.PathValue("repo"), pathIndex(r, "id"), opt)
		reply(w, l, err)
	})
	mux.HandleFunc("DELETE "+repoLabels+"/{id}", func(w http.ResponseWriter, r *http.Request) {
		replyEmpty(w, f.DeleteRepoLabel(r.Context(), r.PathValue("owner"), r.PathValue("repo"), pathIndex(r, "id")))
	})

	mux.HandleFunc("GET "+orgLabels, func(w http.ResponseWriter, r *http.Request) {
		lo := listOptions(r)
		out, info, err := f.ListOrgLabels(r.Context(), r.PathValue("org"), lo.Page, lo.PageSize)
		replyPaged(w, r, out, info, err)
	})
	mux.HandleFunc("POST "+orgLabels, func(w http.ResponseWriter, r *http.Request) {
		var opt forge.LabelCreate
		if !decode(w, r, &opt) {
			return
		}
		l, err := f.CreateOrgLabel(r.Context(), r.PathValue("org"), opt)
		replyStatus(w, http.StatusCreated, l, err)
	})
	mux.HandleFunc("GET "+orgLabels+"/{id}", func(w http.ResponseWriter, r *http.Request) {
		l, err := f.GetOrgLabel(r.Context(), r.PathValue("org"), pathIndex(r, "id"))
		reply(w, l, err)
	})
	mux.HandleFunc("PATCH "+orgLabels+"/{id}", func(w http.ResponseWriter, r *http.Request) {
		var opt forge.LabelPatch
		if !decode(w, r, &opt) {
			return
		}
		l, err := f.EditOrgLabel(r.Context(), r.PathValue("org"), pathIndex(r, "id"), opt)
		reply(w, l, err)
	})
	mux.HandleFunc("DELETE "+orgLabels+"/{id}", func(w http.ResponseWriter, r *http.Request) {
		replyEmpty(w, f.DeleteOrgLabel(r.Context(), r.PathValue("org"), pathIndex(r, "id")))
	})
	mux.HandleFunc("GET /api/v1/orgs/{org}/repos", func(w http.ResponseWriter, r *http.Request) {
		lo := listOptions(r)
		names, err := f.ListOrgRepoNames(r.Context(), r.PathValue("org"), lo.Page, lo.PageSize)
		repos := make([]*forgejo_sdk.Repository, 0, len(names))
		for _, n := range names {
			repos = append(repos, &forgejo_sdk.Repository{Name: n})
		}
		reply(w, repos, err)
	})

	const issue = "/api/v1/repos/{owner}/{repo}/issues/{index}"
	mux.HandleFunc("POST "+issue+"/labels", func(w http.ResponseWriter, r *http.Request) {
		var opt forgejo_sdk.IssueLabelsOption
		if !decode(w, r, &opt) {
			return
		}
		err := f.AddIssueLabels(r.Context(), r.PathValue("owner"), r.PathValue("repo"), pathIndex(r, "index"), opt.Labels)
		reply(w, []*forgejo_sdk.Label{}, err)
	})
	mux.HandleFunc("DELETE "+issue+"/labels/{id}", func(w http.ResponseWriter, r *http.Request) {
		replyEmpty(w, f.RemoveIssueLabel(r.Context(), r.PathValue("owner"), r.PathValue("repo"), pathIndex(r, "index"), pathIndex(r, "id")))
	})
	mux.HandleFunc("POST "+issue+"/dependencies", func(w http.ResponseWriter, r *http.Request) {
		var meta issueMeta
		if !decode(w, r, &meta) {
			return
		}
		err := f.AddIssueDependency(r.Context(), r.PathValue("owner"), r.PathValue("repo"), pathIndex(r, "index"), meta.ref())
		replyStatusEmpty(w, http.StatusCreated, err)
	})
}
