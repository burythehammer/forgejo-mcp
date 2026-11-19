// SPDX-License-Identifier: GPL-3.0-or-later

package forgetest

import (
	"net/http"
	"strconv"
)

// stubRevisionPageSize is the fixed history page size the live server used.
const stubRevisionPageSize = 2

const wikiBase = "/api/v1/repos/{owner}/{repo}/wiki"

// wikiWrite is the body of a wiki create or edit.
type wikiWrite struct {
	Title         string `json:"title"`
	ContentBase64 string `json:"content_base64"`
	Message       string `json:"message"`
}

// mountWiki serves the wiki endpoints from the stub's Fake. Path values reach
// the Fake already percent-decoded, as Forgejo hands them to its handlers.
func (s *Stub) mountWiki(mux *http.ServeMux) {
	f := s.Fake

	mux.HandleFunc("GET "+wikiBase+"/pages", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		page, _ := strconv.Atoi(q.Get("page"))
		limit, _ := strconv.Atoi(q.Get("limit"))
		out, err := f.ListWikiPages(r.Context(), r.PathValue("owner"), r.PathValue("repo"), page, limit)
		if err != nil {
			writeErr(w, err)
			return
		}
		if out.TotalCount != nil {
			w.Header().Set("X-Total-Count", strconv.Itoa(*out.TotalCount))
		}
		writeJSON(w, http.StatusOK, out.Pages)
	})
	mux.HandleFunc("GET "+wikiBase+"/page/{name}", func(w http.ResponseWriter, r *http.Request) {
		p, err := f.GetWikiPage(r.Context(), r.PathValue("owner"), r.PathValue("repo"), r.PathValue("name"))
		reply(w, p, err)
	})
	mux.HandleFunc("GET "+wikiBase+"/revisions/{name}", func(w http.ResponseWriter, r *http.Request) {
		// Forgejo 16.0.4 ignores the limit query and pages the history by a
		// fixed size, and ends each commit message with a newline.
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		out, err := f.GetWikiPageRevisions(r.Context(), r.PathValue("owner"), r.PathValue("repo"), r.PathValue("name"), page, stubRevisionPageSize)
		if out != nil {
			for i := range out.Commits {
				out.Commits[i].Message += "\n"
			}
		}
		reply(w, out, err)
	})
	mux.HandleFunc("POST "+wikiBase+"/new", func(w http.ResponseWriter, r *http.Request) {
		var body wikiWrite
		if !decode(w, r, &body) {
			return
		}
		p, err := f.CreateWikiPage(r.Context(), r.PathValue("owner"), r.PathValue("repo"), body.Title, body.ContentBase64, body.Message)
		replyStatus(w, http.StatusCreated, p, err)
	})
	mux.HandleFunc("PATCH "+wikiBase+"/page/{name}", func(w http.ResponseWriter, r *http.Request) {
		var body wikiWrite
		if !decode(w, r, &body) {
			return
		}
		p, err := f.EditWikiPage(r.Context(), r.PathValue("owner"), r.PathValue("repo"), r.PathValue("name"), body.Title, body.ContentBase64, body.Message)
		reply(w, p, err)
	})
	mux.HandleFunc("DELETE "+wikiBase+"/page/{name}", func(w http.ResponseWriter, r *http.Request) {
		replyEmpty(w, f.DeleteWikiPage(r.Context(), r.PathValue("owner"), r.PathValue("repo"), r.PathValue("name")))
	})
}
