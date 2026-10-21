// SPDX-License-Identifier: GPL-3.0-or-later

package forgetest

import (
	"net/http"
	"strconv"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

const actionsBase = "/api/v1/repos/{owner}/{repo}/actions"

// mountActions serves the Actions endpoints from the stub's Fake.
func (s *Stub) mountActions(mux *http.ServeMux) {
	f := s.Fake
	repo := func(r *http.Request) (string, string) { return r.PathValue("owner"), r.PathValue("repo") }

	mux.HandleFunc("POST "+actionsBase+"/workflows/{workflow}/dispatches", func(w http.ResponseWriter, r *http.Request) {
		var opt forgejo_sdk.DispatchWorkflowOption
		if !decode(w, r, &opt) {
			return
		}
		o, n := repo(r)
		replyEmpty(w, f.DispatchWorkflow(r.Context(), o, n, r.PathValue("workflow"), opt))
	})
	mux.HandleFunc("GET "+actionsBase+"/runs", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		num, _ := strconv.ParseInt(q.Get("run_number"), 10, 64)
		o, n := repo(r)
		out, err := f.ListWorkflowRuns(r.Context(), o, n, forgejo_sdk.ListActionRunsOption{
			ListOptions: listOptions(r), Status: q.Get("status"), Event: q.Get("event"), RunNumber: num, HeadSHA: q.Get("head_sha"),
		})
		reply(w, out, err)
	})
	mux.HandleFunc("GET "+actionsBase+"/runs/{id}", func(w http.ResponseWriter, r *http.Request) {
		o, n := repo(r)
		out, err := f.GetWorkflowRun(r.Context(), o, n, pathIndex(r, "id"))
		reply(w, out, err)
	})
	mux.HandleFunc("POST "+actionsBase+"/runs/{id}/cancel", func(w http.ResponseWriter, r *http.Request) {
		o, n := repo(r)
		replyEmpty(w, f.CancelWorkflowRun(r.Context(), o, n, pathIndex(r, "id")))
	})
	mux.HandleFunc("DELETE "+actionsBase+"/runs/{id}", func(w http.ResponseWriter, r *http.Request) {
		o, n := repo(r)
		replyEmpty(w, f.DeleteWorkflowRun(r.Context(), o, n, pathIndex(r, "id")))
	})
	mux.HandleFunc("GET "+actionsBase+"/runs/{id}/jobs", func(w http.ResponseWriter, r *http.Request) {
		o, n := repo(r)
		out, err := f.ListRunJobs(r.Context(), o, n, pathIndex(r, "id"))
		reply(w, out, err)
	})
	mux.HandleFunc("GET "+actionsBase+"/jobs/{id}/logs", func(w http.ResponseWriter, r *http.Request) {
		attempt, _ := strconv.ParseInt(r.URL.Query().Get("attempt"), 10, 64)
		o, n := repo(r)
		// The size cap is the adapter's to enforce; the stub serves what the
		// range selects.
		out, err := f.GetJobLog(r.Context(), o, n, pathIndex(r, "id"), attempt, r.Header.Get("Range"), 1<<40)
		if err != nil {
			writeErr(w, err)
			return
		}
		w.Header().Set("Content-Type", out.ContentType)
		if out.ContentRange != "" {
			w.Header().Set("Content-Range", out.ContentRange)
		}
		w.WriteHeader(out.StatusCode)
		_, _ = w.Write(out.Body)
	})
	mux.HandleFunc("GET "+actionsBase+"/runs/{id}/artifacts", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		pg, _ := strconv.Atoi(q.Get("page"))
		lim, _ := strconv.Atoi(q.Get("limit"))
		o, n := repo(r)
		out, err := f.ListRunArtifacts(r.Context(), o, n, pathIndex(r, "id"), forge.ArtifactListOptions{Page: pg, Limit: lim, Name: q.Get("name")})
		if err != nil {
			writeErr(w, err)
			return
		}
		if out.TotalCount != nil {
			w.Header().Set("X-Total-Count", strconv.Itoa(*out.TotalCount))
		}
		writeJSON(w, http.StatusOK, out.Artifacts)
	})
	mux.HandleFunc("GET "+actionsBase+"/artifacts/{id}", func(w http.ResponseWriter, r *http.Request) {
		o, n := repo(r)
		out, err := f.GetArtifact(r.Context(), o, n, pathIndex(r, "id"))
		reply(w, out, err)
	})

	mux.HandleFunc("GET "+actionsBase+"/secrets", func(w http.ResponseWriter, r *http.Request) {
		o, n := repo(r)
		out, err := f.ListRepoActionSecrets(r.Context(), o, n, listOptions(r))
		reply(w, out, err)
	})
	mux.HandleFunc("PUT "+actionsBase+"/secrets/{name}", func(w http.ResponseWriter, r *http.Request) {
		var opt forgejo_sdk.CreateSecretOption
		if !decode(w, r, &opt) {
			return
		}
		opt.Name = r.PathValue("name")
		o, n := repo(r)
		replyStatusEmpty(w, http.StatusCreated, f.SetRepoActionSecret(r.Context(), o, n, opt))
	})
	mux.HandleFunc("DELETE "+actionsBase+"/secrets/{name}", func(w http.ResponseWriter, r *http.Request) {
		o, n := repo(r)
		replyEmpty(w, f.DeleteRepoActionSecret(r.Context(), o, n, r.PathValue("name")))
	})
	const orgSecrets = "/api/v1/orgs/{org}/actions/secrets"
	mux.HandleFunc("GET "+orgSecrets, func(w http.ResponseWriter, r *http.Request) {
		out, err := f.ListOrgActionSecrets(r.Context(), r.PathValue("org"), listOptions(r))
		reply(w, out, err)
	})
	mux.HandleFunc("PUT "+orgSecrets+"/{name}", func(w http.ResponseWriter, r *http.Request) {
		var opt forgejo_sdk.CreateSecretOption
		if !decode(w, r, &opt) {
			return
		}
		opt.Name = r.PathValue("name")
		replyStatusEmpty(w, http.StatusCreated, f.SetOrgActionSecret(r.Context(), r.PathValue("org"), opt))
	})
	mux.HandleFunc("DELETE "+orgSecrets+"/{name}", func(w http.ResponseWriter, r *http.Request) {
		replyEmpty(w, f.DeleteOrgActionSecret(r.Context(), r.PathValue("org"), r.PathValue("name")))
	})
}
