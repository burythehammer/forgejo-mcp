// SPDX-License-Identifier: GPL-3.0-or-later

package forgetest

import (
	"fmt"
	"net/http"
	"strconv"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
)

// Forgejo's wire shapes for packages. The port's types carry no JSON tags, so
// the stub translates, nesting the repository as Forgejo does.
type packageWire struct {
	ID         int64  `json:"id"`
	Type       string `json:"type"`
	Name       string `json:"name"`
	Version    string `json:"version"`
	HTMLURL    string `json:"html_url"`
	CreatedAt  string `json:"created_at"`
	Repository *struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

type packageFileWire struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

func packageToWire(v forge.PackageVersion) packageWire {
	w := packageWire{ID: v.ID, Type: v.Type, Name: v.Name, Version: v.Version, HTMLURL: v.HTMLURL, CreatedAt: v.CreatedAt}
	if v.Repository != "" {
		w.Repository = &struct {
			FullName string `json:"full_name"`
		}{v.Repository}
	}
	return w
}

// mountPackages serves the package endpoints from the stub's Fake.
func (s *Stub) mountPackages(mux *http.ServeMux) {
	f := s.Fake
	const version = "/api/v1/packages/{owner}/{type}/{name}/{version}"

	mux.HandleFunc("GET /api/v1/packages/{owner}", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		page, _ := strconv.Atoi(q.Get("page"))
		limit, _ := strconv.Atoi(q.Get("limit"))
		out, err := f.ListPackages(r.Context(), r.PathValue("owner"), forge.ListPackagesOptions{
			Type: q.Get("type"), Query: q.Get("q"), Page: page, Limit: limit,
		})
		if err != nil {
			writeErr(w, err)
			return
		}
		if out.TotalCount != nil {
			w.Header().Set("X-Total-Count", strconv.Itoa(*out.TotalCount))
		}
		if out.HasNext {
			w.Header().Set("Link", fmt.Sprintf(`<%s/api/v1/packages/%s?page=%d&limit=%d>; rel="next"`, s.URL, r.PathValue("owner"), page+1, limit))
		}
		rows := make([]packageWire, 0, len(out.Packages))
		for _, p := range out.Packages {
			rows = append(rows, packageToWire(p))
		}
		writeJSON(w, http.StatusOK, rows)
	})
	mux.HandleFunc("GET "+version, func(w http.ResponseWriter, r *http.Request) {
		p, err := f.GetPackage(r.Context(), r.PathValue("owner"), r.PathValue("type"), r.PathValue("name"), r.PathValue("version"))
		if err != nil {
			writeErr(w, err)
			return
		}
		writeJSON(w, http.StatusOK, packageToWire(*p))
	})
	mux.HandleFunc("DELETE "+version, func(w http.ResponseWriter, r *http.Request) {
		replyEmpty(w, f.DeletePackage(r.Context(), r.PathValue("owner"), r.PathValue("type"), r.PathValue("name"), r.PathValue("version")))
	})
	mux.HandleFunc("GET "+version+"/files", func(w http.ResponseWriter, r *http.Request) {
		files, err := f.ListPackageFiles(r.Context(), r.PathValue("owner"), r.PathValue("type"), r.PathValue("name"), r.PathValue("version"))
		if err != nil {
			writeErr(w, err)
			return
		}
		rows := make([]packageFileWire, 0, len(files))
		for _, p := range files {
			rows = append(rows, packageFileWire(p))
		}
		writeJSON(w, http.StatusOK, rows)
	})
}
