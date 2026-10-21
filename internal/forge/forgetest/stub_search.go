// SPDX-License-Identifier: GPL-3.0-or-later

package forgetest

import (
	"net/http"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// mountSearch serves the search endpoints from the stub's Fake.
func (s *Stub) mountSearch(mux *http.ServeMux) {
	f := s.Fake

	mux.HandleFunc("GET /api/v1/users/search", func(w http.ResponseWriter, r *http.Request) {
		out, err := f.SearchUsers(r.Context(), forgejo_sdk.SearchUsersOption{ListOptions: listOptions(r), KeyWord: r.URL.Query().Get("q")})
		replyStatus(w, http.StatusOK, map[string]any{"ok": true, "data": out}, err)
	})
	mux.HandleFunc("GET /api/v1/orgs/{org}/teams/search", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		out, err := f.SearchOrgTeams(r.Context(), r.PathValue("org"), forgejo_sdk.SearchTeamsOptions{
			ListOptions: listOptions(r), Query: q.Get("q"), IncludeDescription: q.Get("include_desc") == "true",
		})
		replyStatus(w, http.StatusOK, map[string]any{"ok": true, "data": out}, err)
	})
	mux.HandleFunc("GET /api/v1/repos/search", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		out, err := f.SearchRepos(r.Context(), forgejo_sdk.SearchRepoOptions{
			ListOptions: listOptions(r), Keyword: q.Get("q"), KeywordInDescription: q.Get("includeDesc") == "true",
			Sort: q.Get("sort"), Order: q.Get("order"),
		})
		replyStatus(w, http.StatusOK, map[string]any{"ok": true, "data": out}, err)
	})
}
