// SPDX-License-Identifier: GPL-3.0-or-later

package forgetest

import (
	"net/http"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// mountOrgs serves the organization endpoints from the stub's Fake.
func (s *Stub) mountOrgs(mux *http.ServeMux) {
	f := s.Fake
	const orgBase = "/api/v1/orgs/{org}"

	mux.HandleFunc("POST /api/v1/orgs", func(w http.ResponseWriter, r *http.Request) {
		var opt forgejo_sdk.CreateOrgOption
		if !decode(w, r, &opt) {
			return
		}
		org, err := f.CreateOrg(r.Context(), opt)
		replyStatus(w, http.StatusCreated, org, err)
	})
	mux.HandleFunc("GET "+orgBase, func(w http.ResponseWriter, r *http.Request) {
		org, err := f.GetOrg(r.Context(), r.PathValue("org"))
		reply(w, org, err)
	})
	mux.HandleFunc("PATCH "+orgBase, func(w http.ResponseWriter, r *http.Request) {
		var opt forgejo_sdk.EditOrgOption
		if !decode(w, r, &opt) {
			return
		}
		err := f.EditOrg(r.Context(), r.PathValue("org"), opt)
		replyStatusEmpty(w, http.StatusOK, err)
	})
	mux.HandleFunc("DELETE "+orgBase, func(w http.ResponseWriter, r *http.Request) {
		replyEmpty(w, f.DeleteOrg(r.Context(), r.PathValue("org")))
	})
	mux.HandleFunc("GET /api/v1/user/orgs", func(w http.ResponseWriter, r *http.Request) {
		out, err := f.ListMyOrgs(r.Context(), forgejo_sdk.ListOrgsOptions{ListOptions: listOptions(r)})
		reply(w, out, err)
	})
	mux.HandleFunc("GET /api/v1/users/{user}/orgs", func(w http.ResponseWriter, r *http.Request) {
		out, err := f.ListUserOrgs(r.Context(), r.PathValue("user"), forgejo_sdk.ListOrgsOptions{ListOptions: listOptions(r)})
		reply(w, out, err)
	})

	mux.HandleFunc("GET "+orgBase+"/members", func(w http.ResponseWriter, r *http.Request) {
		out, err := f.ListOrgMembers(r.Context(), r.PathValue("org"), forgejo_sdk.ListOrgMembershipOption{ListOptions: listOptions(r)})
		reply(w, out, err)
	})
	// Membership is a status: 204 a member, 404 not.
	mux.HandleFunc("GET "+orgBase+"/members/{user}", func(w http.ResponseWriter, r *http.Request) {
		is, err := f.CheckOrgMembership(r.Context(), r.PathValue("org"), r.PathValue("user"))
		switch {
		case err != nil:
			writeErr(w, err)
		case is:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	mux.HandleFunc("DELETE "+orgBase+"/members/{user}", func(w http.ResponseWriter, r *http.Request) {
		replyEmpty(w, f.RemoveOrgMember(r.Context(), r.PathValue("org"), r.PathValue("user")))
	})

	mux.HandleFunc("GET "+orgBase+"/teams", func(w http.ResponseWriter, r *http.Request) {
		out, err := f.ListOrgTeams(r.Context(), r.PathValue("org"), forgejo_sdk.ListTeamsOptions{ListOptions: listOptions(r)})
		reply(w, out, err)
	})
	mux.HandleFunc("POST "+orgBase+"/teams", func(w http.ResponseWriter, r *http.Request) {
		var opt forgejo_sdk.CreateTeamOption
		if !decode(w, r, &opt) {
			return
		}
		team, err := f.CreateTeam(r.Context(), r.PathValue("org"), opt)
		replyStatus(w, http.StatusCreated, team, err)
	})
	const teamBase = "/api/v1/teams/{id}"
	mux.HandleFunc("PUT "+teamBase+"/members/{user}", func(w http.ResponseWriter, r *http.Request) {
		replyEmpty(w, f.AddTeamMember(r.Context(), pathIndex(r, "id"), r.PathValue("user")))
	})
	mux.HandleFunc("DELETE "+teamBase+"/members/{user}", func(w http.ResponseWriter, r *http.Request) {
		replyEmpty(w, f.RemoveTeamMember(r.Context(), pathIndex(r, "id"), r.PathValue("user")))
	})
	mux.HandleFunc("PUT "+teamBase+"/repos/{org}/{repo}", func(w http.ResponseWriter, r *http.Request) {
		replyEmpty(w, f.AddTeamRepository(r.Context(), pathIndex(r, "id"), r.PathValue("org"), r.PathValue("repo")))
	})
	mux.HandleFunc("DELETE "+teamBase+"/repos/{org}/{repo}", func(w http.ResponseWriter, r *http.Request) {
		replyEmpty(w, f.RemoveTeamRepository(r.Context(), pathIndex(r, "id"), r.PathValue("org"), r.PathValue("repo")))
	})
}
