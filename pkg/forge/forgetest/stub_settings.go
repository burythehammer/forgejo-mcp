// SPDX-License-Identifier: GPL-3.0-or-later

package forgetest

import (
	"net/http"
	"strconv"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"

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

// hookWire is a hook as Forgejo serves it: branch_filter sits at the top level,
// not inside config, and the SDK's Hook type has no field for it. Embedding
// the SDK type keeps its other fields as they are on the wire.
type hookWire struct {
	forgejo_sdk.Hook
	BranchFilter string `json:"branch_filter"`
}

// wireHook moves the Fake's branch filter, which it keeps in Config, to the
// top level, as Forgejo serves it. A nil hook stays nil.
func wireHook(h *forgejo_sdk.Hook) *hookWire {
	if h == nil {
		return nil
	}
	c := *h
	c.Config = make(map[string]string, len(h.Config))
	filter := ""
	for k, v := range h.Config {
		if k == "branch_filter" {
			filter = v
			continue
		}
		c.Config[k] = v
	}
	return &hookWire{Hook: c, BranchFilter: filter}
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
		wired := make([]*hookWire, 0, len(out.Hooks))
		for _, h := range out.Hooks {
			wired = append(wired, wireHook(h))
		}
		writeJSON(w, http.StatusOK, wired)
	})
	mux.HandleFunc("POST "+hooksURL, func(w http.ResponseWriter, r *http.Request) {
		var opt forgejo_sdk.CreateHookOption
		if !decode(w, r, &opt) {
			return
		}
		h, err := f.CreateRepoHook(r.Context(), r.PathValue("owner"), r.PathValue("repo"), opt)
		replyStatus(w, http.StatusCreated, wireHook(h), err)
	})
	mux.HandleFunc("GET "+hooksURL+"/{id}", func(w http.ResponseWriter, r *http.Request) {
		h, err := f.GetRepoHook(r.Context(), r.PathValue("owner"), r.PathValue("repo"), pathIndex(r, "id"))
		reply(w, wireHook(h), err)
	})
	mux.HandleFunc("PATCH "+hooksURL+"/{id}", func(w http.ResponseWriter, r *http.Request) {
		// Presence matters: an absent key stays nil, an explicit "" stays a
		// non-nil value, as Forgejo reads a PATCH body. forge.HookEdit's json
		// tags name the keys, and encoding/json keeps that presence.
		var opt forge.HookEdit
		if !decode(w, r, &opt) {
			return
		}
		// Real Forgejo clears a branch filter that a PATCH omits, so the stub
		// does too. The Fake itself keeps the port's "unset is unchanged" rule.
		if opt.BranchFilter == nil {
			cleared := ""
			opt.BranchFilter = &cleared
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
