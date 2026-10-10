// SPDX-License-Identifier: GPL-3.0-or-later

package forgetest

import (
	"net/http"
	"strconv"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

const (
	bpBase   = "/api/v1/repos/{owner}/{repo}/branch_protections"
	hooksURL = "/api/v1/repos/{owner}/{repo}/hooks"
)

// mountBranchProtections serves the branch-protection endpoints from the Fake.
func (s *Stub) mountBranchProtections(mux *http.ServeMux) {
	f := s.Fake

	mux.HandleFunc("GET "+bpBase, func(w http.ResponseWriter, r *http.Request) {
		out, err := f.ListBranchProtections(r.Context(), r.PathValue("owner"), r.PathValue("repo"),
			forgejo_sdk.ListBranchProtectionsOptions{ListOptions: listOptions(r)})
		reply(w, out, err)
	})
	mux.HandleFunc("POST "+bpBase, func(w http.ResponseWriter, r *http.Request) {
		var opt forgejo_sdk.CreateBranchProtectionOption
		if !decode(w, r, &opt) {
			return
		}
		bp, err := f.CreateBranchProtection(r.Context(), r.PathValue("owner"), r.PathValue("repo"), opt)
		replyStatus(w, http.StatusCreated, bp, err)
	})
	mux.HandleFunc("GET "+bpBase+"/{rule}", func(w http.ResponseWriter, r *http.Request) {
		bp, err := f.GetBranchProtection(r.Context(), r.PathValue("owner"), r.PathValue("repo"), r.PathValue("rule"))
		reply(w, bp, err)
	})
	mux.HandleFunc("PATCH "+bpBase+"/{rule}", func(w http.ResponseWriter, r *http.Request) {
		var opt forgejo_sdk.EditBranchProtectionOption
		if !decode(w, r, &opt) {
			return
		}
		bp, err := f.EditBranchProtection(r.Context(), r.PathValue("owner"), r.PathValue("repo"), r.PathValue("rule"), opt)
		reply(w, bp, err)
	})
	mux.HandleFunc("DELETE "+bpBase+"/{rule}", func(w http.ResponseWriter, r *http.Request) {
		replyEmpty(w, f.DeleteBranchProtection(r.Context(), r.PathValue("owner"), r.PathValue("repo"), r.PathValue("rule")))
	})
}

// mountHooks serves the repository-webhook endpoints from the Fake.
func (s *Stub) mountHooks(mux *http.ServeMux) {
	f := s.Fake

	mux.HandleFunc("GET "+hooksURL, func(w http.ResponseWriter, r *http.Request) {
		out, err := f.ListRepoHooks(r.Context(), r.PathValue("owner"), r.PathValue("repo"),
			forgejo_sdk.ListHooksOptions{ListOptions: listOptions(r)})
		if err != nil {
			writeErr(w, err)
			return
		}
		if out.TotalCount != nil {
			w.Header().Set("X-Total-Count", strconv.Itoa(*out.TotalCount))
		}
		writeJSON(w, http.StatusOK, out.Hooks)
	})
	mux.HandleFunc("POST "+hooksURL, func(w http.ResponseWriter, r *http.Request) {
		var opt forgejo_sdk.CreateHookOption
		if !decode(w, r, &opt) {
			return
		}
		h, err := f.CreateRepoHook(r.Context(), r.PathValue("owner"), r.PathValue("repo"), opt)
		replyStatus(w, http.StatusCreated, h, err)
	})
	mux.HandleFunc("GET "+hooksURL+"/{id}", func(w http.ResponseWriter, r *http.Request) {
		h, err := f.GetRepoHook(r.Context(), r.PathValue("owner"), r.PathValue("repo"), pathIndex(r, "id"))
		reply(w, h, err)
	})
	mux.HandleFunc("PATCH "+hooksURL+"/{id}", func(w http.ResponseWriter, r *http.Request) {
		var opt forgejo_sdk.EditHookOption
		if !decode(w, r, &opt) {
			return
		}
		// Forgejo answers an edit with the hook; the SDK ignores the body.
		err := f.EditRepoHook(r.Context(), r.PathValue("owner"), r.PathValue("repo"), pathIndex(r, "id"), opt)
		replyEmpty(w, err)
	})
	mux.HandleFunc("DELETE "+hooksURL+"/{id}", func(w http.ResponseWriter, r *http.Request) {
		replyEmpty(w, f.DeleteRepoHook(r.Context(), r.PathValue("owner"), r.PathValue("repo"), pathIndex(r, "id")))
	})
	mux.HandleFunc("POST "+hooksURL+"/{id}/tests", func(w http.ResponseWriter, r *http.Request) {
		replyEmpty(w, f.TestRepoHook(r.Context(), r.PathValue("owner"), r.PathValue("repo"), pathIndex(r, "id")))
	})
}
