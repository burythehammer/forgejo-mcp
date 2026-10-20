// SPDX-License-Identifier: GPL-3.0-or-later

package forgetest

import (
	"encoding/json"
	"net/http"
	"strconv"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

const repoBase = "/api/v1/repos/{owner}/{repo}"

// mountRepoFiles serves the file, tree, commit and commit-status endpoints from
// the stub's Fake.
func (s *Stub) mountRepoFiles(mux *http.ServeMux) {
	f := s.Fake
	own := func(r *http.Request) (string, string) { return r.PathValue("owner"), r.PathValue("repo") }

	mux.HandleFunc("GET "+repoBase+"/raw/{path...}", func(w http.ResponseWriter, r *http.Request) {
		o, n := own(r)
		b, err := f.GetFile(r.Context(), o, n, r.URL.Query().Get("ref"), r.PathValue("path"))
		if err != nil {
			writeErr(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(b)
	})
	// One path serves a file (object) or a directory (array); the SDK decides
	// which it expects, so the stub answers with whichever the fake can.
	mux.HandleFunc("GET "+repoBase+"/contents/{path...}", func(w http.ResponseWriter, r *http.Request) {
		o, n := own(r)
		ref, p := r.URL.Query().Get("ref"), r.PathValue("path")
		if list, err := f.ListContents(r.Context(), o, n, ref, p); err == nil {
			writeJSON(w, http.StatusOK, list)
			return
		} else if forge.StatusCode(err) != 0 {
			writeErr(w, err)
			return
		}
		file, err := f.GetContents(r.Context(), o, n, ref, p)
		reply(w, file, err)
	})
	mux.HandleFunc("GET "+repoBase+"/contents", func(w http.ResponseWriter, r *http.Request) {
		o, n := own(r)
		list, err := f.ListContents(r.Context(), o, n, r.URL.Query().Get("ref"), "")
		reply(w, list, err)
	})
	mux.HandleFunc("POST "+repoBase+"/contents", func(w http.ResponseWriter, r *http.Request) {
		var opt forge.ChangeFilesOptions
		if !decode(w, r, &opt) {
			return
		}
		o, n := own(r)
		out, err := f.ChangeFiles(r.Context(), o, n, opt)
		replyStatus(w, http.StatusCreated, out, err)
	})
	mux.HandleFunc("POST "+repoBase+"/contents/{path...}", func(w http.ResponseWriter, r *http.Request) {
		var opt forgejo_sdk.CreateFileOptions
		if !decode(w, r, &opt) {
			return
		}
		o, n := own(r)
		out, err := f.CreateFile(r.Context(), o, n, r.PathValue("path"), opt)
		replyStatus(w, http.StatusCreated, out, err)
	})
	mux.HandleFunc("PUT "+repoBase+"/contents/{path...}", func(w http.ResponseWriter, r *http.Request) {
		var opt forgejo_sdk.UpdateFileOptions
		if !decode(w, r, &opt) {
			return
		}
		o, n := own(r)
		out, err := f.UpdateFile(r.Context(), o, n, r.PathValue("path"), opt)
		reply(w, out, err)
	})
	mux.HandleFunc("DELETE "+repoBase+"/contents/{path...}", func(w http.ResponseWriter, r *http.Request) {
		var opt forgejo_sdk.DeleteFileOptions
		if !decode(w, r, &opt) {
			return
		}
		o, n := own(r)
		err := f.DeleteFile(r.Context(), o, n, r.PathValue("path"), opt)
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{})
	})
	mux.HandleFunc("GET "+repoBase+"/git/trees/{ref}", func(w http.ResponseWriter, r *http.Request) {
		o, n := own(r)
		q := r.URL.Query()
		out, err := f.GetTree(r.Context(), o, n, r.PathValue("ref"), forgejo_sdk.GetTreesOptions{
			Recursive: q.Get("recursive") == "1" || q.Get("recursive") == "true", ListOptions: listOptions(r),
		})
		reply(w, out, err)
	})
	mux.HandleFunc("GET "+repoBase+"/git/commits/{sha}", func(w http.ResponseWriter, r *http.Request) {
		o, n := own(r)
		out, err := f.GetCommit(r.Context(), o, n, r.PathValue("sha"))
		reply(w, out, err)
	})
	mux.HandleFunc("GET "+repoBase+"/commits", func(w http.ResponseWriter, r *http.Request) {
		o, n := own(r)
		q := r.URL.Query()
		out, err := f.ListCommits(r.Context(), o, n, forgejo_sdk.ListCommitOptions{
			ListOptions: listOptions(r), SHA: q.Get("sha"), Path: q.Get("path"),
		})
		reply(w, out, err)
	})
	mux.HandleFunc("GET "+repoBase+"/commits/{sha}/statuses", func(w http.ResponseWriter, r *http.Request) {
		o, n := own(r)
		out, err := f.ListCommitStatuses(r.Context(), o, n, r.PathValue("sha"), forgejo_sdk.ListStatusesOption{ListOptions: listOptions(r)})
		if err != nil {
			writeErr(w, err)
			return
		}
		if out.TotalCount != nil {
			w.Header().Set("X-Total-Count", strconv.Itoa(*out.TotalCount))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out.Statuses)
	})
}
