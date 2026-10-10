// SPDX-License-Identifier: GPL-3.0-or-later

package forgetest

import (
	"net/http"
	"strconv"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

const releasesBase = "/api/v1/repos/{owner}/{repo}/releases"

func optBool(r *http.Request, name string) *bool {
	v := r.URL.Query().Get(name)
	if v == "" {
		return nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return nil
	}
	return &b
}

// mountReleases serves the release endpoints, including release assets, from
// the stub's Fake.
func (s *Stub) mountReleases(mux *http.ServeMux) {
	f := s.Fake

	mux.HandleFunc("GET "+releasesBase, func(w http.ResponseWriter, r *http.Request) {
		out, err := f.ListReleases(r.Context(), r.PathValue("owner"), r.PathValue("repo"), forgejo_sdk.ListReleasesOptions{
			ListOptions:  listOptions(r),
			IsDraft:      optBool(r, "draft"),
			IsPreRelease: optBool(r, "pre-release"),
		})
		reply(w, out, err)
	})
	mux.HandleFunc("POST "+releasesBase, func(w http.ResponseWriter, r *http.Request) {
		var opt forgejo_sdk.CreateReleaseOption
		if !decode(w, r, &opt) {
			return
		}
		rel, err := f.CreateRelease(r.Context(), r.PathValue("owner"), r.PathValue("repo"), opt)
		replyStatus(w, http.StatusCreated, rel, err)
	})
	mux.HandleFunc("GET "+releasesBase+"/latest", func(w http.ResponseWriter, r *http.Request) {
		rel, err := f.GetLatestRelease(r.Context(), r.PathValue("owner"), r.PathValue("repo"))
		reply(w, rel, err)
	})
	mux.HandleFunc("DELETE "+releasesBase+"/tags/{tag}", func(w http.ResponseWriter, r *http.Request) {
		replyEmpty(w, f.DeleteReleaseByTag(r.Context(), r.PathValue("owner"), r.PathValue("repo"), r.PathValue("tag")))
	})
	mux.HandleFunc("GET "+releasesBase+"/{id}", func(w http.ResponseWriter, r *http.Request) {
		rel, err := f.GetRelease(r.Context(), r.PathValue("owner"), r.PathValue("repo"), pathIndex(r, "id"))
		reply(w, rel, err)
	})
	mux.HandleFunc("PATCH "+releasesBase+"/{id}", func(w http.ResponseWriter, r *http.Request) {
		var opt forgejo_sdk.EditReleaseOption
		if !decode(w, r, &opt) {
			return
		}
		rel, err := f.EditRelease(r.Context(), r.PathValue("owner"), r.PathValue("repo"), pathIndex(r, "id"), opt)
		reply(w, rel, err)
	})
	mux.HandleFunc("DELETE "+releasesBase+"/{id}", func(w http.ResponseWriter, r *http.Request) {
		replyEmpty(w, f.DeleteRelease(r.Context(), r.PathValue("owner"), r.PathValue("repo"), pathIndex(r, "id")))
	})

	assets := releasesBase + "/{id}/assets"
	// "releases/tags/{tag}" and "releases/{id}/assets" overlap, so one route
	// serves both.
	mux.HandleFunc("GET "+releasesBase+"/{id}/{what}", func(w http.ResponseWriter, r *http.Request) {
		owner, repo := r.PathValue("owner"), r.PathValue("repo")
		switch {
		case r.PathValue("id") == "tags":
			rel, err := f.GetReleaseByTag(r.Context(), owner, repo, r.PathValue("what"))
			reply(w, rel, err)
		case r.PathValue("what") == "assets":
			out, err := f.ListReleaseAttachments(r.Context(), owner, repo, pathIndex(r, "id"))
			reply(w, out, err)
		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("POST "+assets, func(w http.ResponseWriter, r *http.Request) {
		up, ok := readUpload(w, r)
		if !ok {
			return
		}
		att, err := f.CreateReleaseAttachment(r.Context(), r.PathValue("owner"), r.PathValue("repo"), pathIndex(r, "id"), up)
		replyStatus(w, http.StatusCreated, att, err)
	})
	mux.HandleFunc("GET "+assets+"/{aid}", func(w http.ResponseWriter, r *http.Request) {
		att, err := f.GetReleaseAttachment(r.Context(), r.PathValue("owner"), r.PathValue("repo"), pathIndex(r, "id"), pathIndex(r, "aid"))
		reply(w, att, err)
	})
	mux.HandleFunc("PATCH "+assets+"/{aid}", func(w http.ResponseWriter, r *http.Request) {
		var opt forgejo_sdk.EditAttachmentOptions
		if !decode(w, r, &opt) {
			return
		}
		att, err := f.EditReleaseAttachment(r.Context(), r.PathValue("owner"), r.PathValue("repo"), pathIndex(r, "id"), pathIndex(r, "aid"), opt.Name)
		reply(w, att, err)
	})
	mux.HandleFunc("DELETE "+assets+"/{aid}", func(w http.ResponseWriter, r *http.Request) {
		replyEmpty(w, f.DeleteReleaseAttachment(r.Context(), r.PathValue("owner"), r.PathValue("repo"), pathIndex(r, "id"), pathIndex(r, "aid")))
	})
}
