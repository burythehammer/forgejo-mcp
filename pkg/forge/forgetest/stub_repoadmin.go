// SPDX-License-Identifier: GPL-3.0-or-later

package forgetest

import (
	"net/http"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

const repoAdminBase = "/api/v1/repos/{owner}/{repo}"

// mountRepoAdmin serves the repository, topic and branch endpoints from the
// stub's Fake.
func (s *Stub) mountRepoAdmin(mux *http.ServeMux) {
	f := s.Fake
	own := func(r *http.Request) (string, string) { return r.PathValue("owner"), r.PathValue("repo") }

	mux.HandleFunc("POST /api/v1/user/repos", func(w http.ResponseWriter, r *http.Request) {
		var opt forgejo_sdk.CreateRepoOption
		if !decode(w, r, &opt) {
			return
		}
		out, err := f.CreateRepo(r.Context(), opt)
		replyStatus(w, http.StatusCreated, out, err)
	})
	mux.HandleFunc("POST /api/v1/orgs/{org}/repos", func(w http.ResponseWriter, r *http.Request) {
		var opt forgejo_sdk.CreateRepoOption
		if !decode(w, r, &opt) {
			return
		}
		out, err := f.CreateOrgRepo(r.Context(), r.PathValue("org"), opt)
		replyStatus(w, http.StatusCreated, out, err)
	})
	mux.HandleFunc("POST "+repoAdminBase+"/forks", func(w http.ResponseWriter, r *http.Request) {
		var opt forgejo_sdk.CreateForkOption
		if !decode(w, r, &opt) {
			return
		}
		o, n := own(r)
		out, err := f.ForkRepo(r.Context(), o, n, opt)
		replyStatus(w, http.StatusAccepted, out, err)
	})
	mux.HandleFunc("GET /api/v1/user/repos", func(w http.ResponseWriter, r *http.Request) {
		out, err := f.ListMyRepos(r.Context(), forgejo_sdk.ListReposOptions{ListOptions: listOptions(r)})
		reply(w, out, err)
	})

	mux.HandleFunc("GET "+repoAdminBase, func(w http.ResponseWriter, r *http.Request) {
		o, n := own(r)
		out, err := f.GetRepo(r.Context(), o, n)
		reply(w, out, err)
	})
	mux.HandleFunc("PATCH "+repoAdminBase, func(w http.ResponseWriter, r *http.Request) {
		var opt forgejo_sdk.EditRepoOption
		if !decode(w, r, &opt) {
			return
		}
		o, n := own(r)
		out, err := f.EditRepo(r.Context(), o, n, opt)
		reply(w, out, err)
	})

	mux.HandleFunc("GET "+repoAdminBase+"/topics", func(w http.ResponseWriter, r *http.Request) {
		o, n := own(r)
		out, err := f.ListRepoTopics(r.Context(), o, n, forgejo_sdk.ListRepoTopicsOptions{ListOptions: listOptions(r)})
		reply(w, map[string][]string{"topics": out}, err)
	})
	mux.HandleFunc("PUT "+repoAdminBase+"/topics", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Topics []string `json:"topics"`
		}
		if !decode(w, r, &body) {
			return
		}
		o, n := own(r)
		replyEmpty(w, f.SetRepoTopics(r.Context(), o, n, body.Topics))
	})
	mux.HandleFunc("PUT "+repoAdminBase+"/topics/{topic}", func(w http.ResponseWriter, r *http.Request) {
		o, n := own(r)
		replyEmpty(w, f.AddRepoTopic(r.Context(), o, n, r.PathValue("topic")))
	})
	mux.HandleFunc("DELETE "+repoAdminBase+"/topics/{topic}", func(w http.ResponseWriter, r *http.Request) {
		o, n := own(r)
		replyEmpty(w, f.DeleteRepoTopic(r.Context(), o, n, r.PathValue("topic")))
	})

	mux.HandleFunc("GET "+repoAdminBase+"/branches", func(w http.ResponseWriter, r *http.Request) {
		o, n := own(r)
		out, err := f.ListBranches(r.Context(), o, n, forgejo_sdk.ListRepoBranchesOptions{ListOptions: listOptions(r)})
		reply(w, out, err)
	})
	mux.HandleFunc("POST "+repoAdminBase+"/branches", func(w http.ResponseWriter, r *http.Request) {
		var opt struct {
			BranchName    string `json:"new_branch_name"`
			OldBranchName string `json:"old_branch_name"`
		}
		if !decode(w, r, &opt) {
			return
		}
		o, n := own(r)
		out, err := f.CreateBranch(r.Context(), o, n, forgejo_sdk.CreateBranchOption{BranchName: opt.BranchName, OldBranchName: opt.OldBranchName})
		replyStatus(w, http.StatusCreated, out, err)
	})
	mux.HandleFunc("DELETE "+repoAdminBase+"/branches/{branch}", func(w http.ResponseWriter, r *http.Request) {
		o, n := own(r)
		ok, err := f.DeleteBranch(r.Context(), o, n, r.PathValue("branch"))
		if err == nil && !ok {
			w.WriteHeader(http.StatusOK)
			return
		}
		replyEmpty(w, err)
	})
}
