// SPDX-License-Identifier: GPL-3.0-or-later

package forgetest

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// readUpload parses a multipart attachment upload into a forge.Upload.
func readUpload(w http.ResponseWriter, r *http.Request) (forge.Upload, bool) {
	file, hdr, err := r.FormFile("attachment")
	if err != nil {
		writeErr(w, errors.New("bad upload: "+err.Error()))
		return forge.Upload{}, false
	}
	// The file is closed when the request ends.
	return forge.Upload{Filename: hdr.Filename, MIMEType: hdr.Header.Get("Content-Type"), Content: file}, true
}

// assetRoute is a parsed issue or comment attachment path.
type assetRoute struct {
	comment bool  // under issues/comments/{id}, otherwise issues/{index}
	parent  int64 // issue index or comment id
	id      int64 // attachment id, when hasID
	hasID   bool
}

// parseAssetRoute reads the part of an attachment path after "issues/":
//
//	{index}/assets[/{id}]  or  comments/{commentID}/assets[/{id}]
//
// The two overlap as route patterns ("issues/comments/7/assets" against
// "issues/3/assets/9"), so one catch-all route dispatches on the segments.
func parseAssetRoute(rest string) (assetRoute, bool) {
	seg := strings.Split(strings.Trim(rest, "/"), "/")
	var rt assetRoute
	if len(seg) > 0 && seg[0] == "comments" {
		rt.comment = true
		seg = seg[1:]
	}
	if len(seg) < 2 || len(seg) > 3 || seg[1] != "assets" {
		return rt, false
	}
	var err error
	if rt.parent, err = strconv.ParseInt(seg[0], 10, 64); err != nil {
		return rt, false
	}
	if len(seg) == 3 {
		if rt.id, err = strconv.ParseInt(seg[2], 10, 64); err != nil {
			return rt, false
		}
		rt.hasID = true
	}
	return rt, true
}

// mountAttachments serves the issue and comment attachment endpoints and the
// download URL of every attachment from the stub's Fake.
func (s *Stub) mountAttachments(mux *http.ServeMux) {
	f := s.Fake
	const issues = "/api/v1/repos/{owner}/{repo}/issues/{rest...}"

	s.assets = map[string]http.HandlerFunc{}
	serve := func(method string, h func(w http.ResponseWriter, r *http.Request, owner, repo string, rt assetRoute)) {
		handler := func(w http.ResponseWriter, r *http.Request) {
			rest := r.PathValue("rest")
			if rest == "" { // reached through the issue stub's two-segment route
				rest = r.PathValue("a") + "/" + r.PathValue("b")
			}
			rt, ok := parseAssetRoute(rest)
			if !ok {
				http.NotFound(w, r)
				return
			}
			h(w, r, r.PathValue("owner"), r.PathValue("repo"), rt)
		}
		s.assets[method] = handler
		mux.HandleFunc(method+" "+issues, handler)
	}

	serve("GET", func(w http.ResponseWriter, r *http.Request, owner, repo string, rt assetRoute) {
		ctx := r.Context()
		switch {
		case rt.hasID && rt.comment:
			att, err := f.GetCommentAttachment(ctx, owner, repo, rt.parent, rt.id)
			reply(w, att, err)
		case rt.hasID:
			att, err := f.GetIssueAttachment(ctx, owner, repo, rt.parent, rt.id)
			reply(w, att, err)
		case rt.comment:
			out, err := f.ListCommentAttachments(ctx, owner, repo, rt.parent)
			reply(w, out, err)
		default:
			out, err := f.ListIssueAttachments(ctx, owner, repo, rt.parent)
			reply(w, out, err)
		}
	})
	serve("POST", func(w http.ResponseWriter, r *http.Request, owner, repo string, rt assetRoute) {
		if rt.hasID {
			http.NotFound(w, r)
			return
		}
		up, ok := readUpload(w, r)
		if !ok {
			return
		}
		var att *forgejo_sdk.Attachment
		var err error
		if rt.comment {
			att, err = f.CreateCommentAttachment(r.Context(), owner, repo, rt.parent, up)
		} else {
			att, err = f.CreateIssueAttachment(r.Context(), owner, repo, rt.parent, up)
		}
		replyStatus(w, http.StatusCreated, att, err)
	})
	serve("PATCH", func(w http.ResponseWriter, r *http.Request, owner, repo string, rt assetRoute) {
		if !rt.hasID {
			http.NotFound(w, r)
			return
		}
		var opt forgejo_sdk.EditAttachmentOptions
		if !decode(w, r, &opt) {
			return
		}
		var att *forgejo_sdk.Attachment
		var err error
		if rt.comment {
			att, err = f.EditCommentAttachment(r.Context(), owner, repo, rt.parent, rt.id, opt.Name)
		} else {
			att, err = f.EditIssueAttachment(r.Context(), owner, repo, rt.parent, rt.id, opt.Name)
		}
		reply(w, att, err)
	})
	serve("DELETE", func(w http.ResponseWriter, r *http.Request, owner, repo string, rt assetRoute) {
		if !rt.hasID {
			http.NotFound(w, r)
			return
		}
		if rt.comment {
			replyEmpty(w, f.DeleteCommentAttachment(r.Context(), owner, repo, rt.parent, rt.id))
		} else {
			replyEmpty(w, f.DeleteIssueAttachment(r.Context(), owner, repo, rt.parent, rt.id))
		}
	})

	// The download URL the Fake issues: BaseURL + /attachments/{uuid}.
	mux.HandleFunc("GET /attachments/{uuid}", func(w http.ResponseWriter, r *http.Request) {
		d, err := f.AttachmentContent(f.BaseURL + "/attachments/" + r.PathValue("uuid"))
		if err != nil {
			writeErr(w, err)
			return
		}
		w.Header().Set("Content-Type", d.ContentType)
		_, _ = w.Write(d.Body)
	})
}
