// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/list"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/model"
	apprepo "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/repo"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge/repogitadapter"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/log"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/resource"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// commitResourceURITemplate carries the {?page,limit,message_page} expansion,
// which is load-bearing: a template registered bare matches only the bare URI,
// so a query-bearing read would fail with "resource not found" (see the labels
// template in internal/operation/issue/resources_label.go).
const commitResourceURITemplate = "forgejo://repo/{owner}/{repo}/commit/{sha}{?page,limit,message_page}"

// RegisterCommitResource registers the forgejo://repo/{owner}/{repo}/commit/{sha}{?page,limit,message_page} resource template.
func RegisterCommitResource(s *server.MCPServer, f forge.Factory) {
	h := handlers{forge: f}
	resource.RegisterTemplate(
		s,
		commitResourceURITemplate,
		"Forgejo Commit",
		h.commitResourceHandler,
		mcp.WithTemplateDescription(
			"Immutable commit metadata for a Forgejo repository commit. "+
				"URI: forgejo://repo/{owner}/{repo}/commit/{sha}{?page,limit,message_page} — sha must be a full 40-character hex SHA. "+
				"Returns JSON metadata plus a text/markdown body sidecar for the commit message. "+
				"The file list is one page at a time: page and limit (default and maximum 30 files) select it, and files_next names the next page when more follow. "+
				"A message longer than 16384 bytes is read one part at a time: message_page (default 1) selects the part, and message_next names the next part when more follow.",
		),
		mcp.WithTemplateMIMEType("application/json"),
	)
	log.Debug("Registered commit resource template")
}

// commitMessageCap is the size of one part of a commit message. A message
// longer than this is read part by part through message_page, so no read
// returns more than one part (docs/design/output-bounding.md, sub-rule 3).
const commitMessageCap = 16 * 1024

// messagePart is the part of a commit message that one read returns.
type messagePart struct {
	text    string
	page    int  // 1-based part number
	parts   int  // number of parts the message has
	cut     bool // the message spans more than one part
	hasNext bool // a part follows this one
}

// messageParts cuts msg into parts of at most commitMessageCap bytes. A part
// ends on a rune boundary, so each part is valid UTF-8 for valid input, and the
// parts join back to msg.
func messageParts(msg string) []string {
	var parts []string
	for len(msg) > 0 {
		n := min(len(msg), commitMessageCap)
		if n < len(msg) {
			back := n
			for back > 0 && !utf8.RuneStart(msg[back]) {
				back--
			}
			if back > 0 {
				n = back
			}
		}
		parts = append(parts, msg[:n])
		msg = msg[n:]
	}
	return parts
}

// commitMessagePart is the part of c's message that page names, read through
// the list module as a list of parts of one row each. A message that fits in
// one part is returned whole, whatever page asks for.
func commitMessagePart(c *model.Commit, page int) messagePart {
	var msg string
	if c.RepoCommit != nil {
		msg = c.RepoCommit.Message
	}
	parts := messageParts(msg)
	if len(parts) <= 1 {
		return messagePart{text: msg, page: 1, parts: len(parts)}
	}
	b := list.Cut(parts, list.Paging{Page: page, Limit: 1})
	return messagePart{
		text:    strings.Join(b.Items, ""),
		page:    page,
		parts:   len(parts),
		cut:     true,
		hasNext: b.HasNext,
	}
}

// commitMessageMarker closes the markdown sidecar of a cut message part. It
// names the next part when one follows.
func commitMessageMarker(p messagePart) string {
	s := fmt.Sprintf("\n\n[message truncated: part %d of %d", p.page, p.parts)
	if p.hasNext {
		s += fmt.Sprintf("; read part %d with message_page=%d", p.page+1, p.page+1)
	}
	return s + "]"
}

// marshalBoundedCommit encodes a copy of c with its message replaced by the
// part msg names and its file list cut to the page pg names. The commit's own
// keys come first. When the message is cut, message_truncated, message_page,
// message_pages and (when a part follows) message_next are added. When the file
// list is cut, files_total, files_page, files_limit and files_truncated are
// added, and files_next names the next page when one follows. A commit that
// fits in its page encodes byte-for-byte as before.
func marshalBoundedCommit(c *model.Commit, pg list.Paging, msg messagePart, nextFiles, nextMsg string) ([]byte, error) {
	cp := *c
	var extra []string
	if msg.cut {
		rc := *cp.RepoCommit
		rc.Message = msg.text
		cp.RepoCommit = &rc
		extra = append(extra,
			`"message_truncated":true`,
			fmt.Sprintf(`"message_page":%d`, msg.page),
			fmt.Sprintf(`"message_pages":%d`, msg.parts),
		)
		if msg.hasNext {
			nb, err := json.Marshal(nextMsg)
			if err != nil {
				return nil, err
			}
			extra = append(extra, `"message_next":`+string(nb))
		}
	}
	total := len(cp.Files)
	bounded := list.Cut(cp.Files, pg)
	rows, hasNext := bounded.Items, bounded.HasNext
	cut := hasNext || pg.Page > 1
	cp.Files = rows
	b, err := json.Marshal(&cp)
	if err != nil {
		return nil, err
	}
	if cut {
		extra = append(extra,
			fmt.Sprintf(`"files_total":%d`, total),
			fmt.Sprintf(`"files_page":%d`, pg.Page),
			fmt.Sprintf(`"files_limit":%d`, pg.Limit),
			fmt.Sprintf(`"files_truncated":%t`, hasNext),
		)
		if hasNext {
			nb, err := json.Marshal(nextFiles)
			if err != nil {
				return nil, err
			}
			extra = append(extra, `"files_next":`+string(nb))
		}
	}
	if len(extra) == 0 {
		return b, nil
	}
	return append(b[:len(b)-1], ","+strings.Join(extra, ",")+"}"...), nil
}

// commitReadURI is the URI of one read of a commit. page and limit select the
// file list; message_page selects the message part and is written only past
// the first part, so the URIs of a file list read at the first part are
// unchanged.
func commitReadURI(owner, repo, sha string, pg list.Paging, msgPage int) string {
	q := fmt.Sprintf("page=%d&limit=%d", pg.Page, pg.Limit)
	if msgPage > 1 {
		q += fmt.Sprintf("&message_page=%d", msgPage)
	}
	return fmt.Sprintf("forgejo://repo/%s/%s/commit/%s?%s",
		url.PathEscape(owner), url.PathEscape(repo), url.PathEscape(sha), q)
}

// messagePageOf reads message_page from uri: the 1-based part of the message to
// read. It is 1 when absent or below 1.
func messagePageOf(uri string) int {
	u, err := url.Parse(uri)
	if err != nil {
		return 1
	}
	if v, err := strconv.Atoi(u.Query().Get("message_page")); err == nil && v >= 1 {
		return v
	}
	return 1
}

func (h handlers) commitResourceHandler(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := req.Params.URI
	params, err := resource.ParseCommit(uri)
	if err != nil {
		return nil, resource.MapForgejoError(uri, err)
	}
	page, limit := resource.PageLimit(req)
	pg := list.Paging{Page: page, Limit: limit}
	msgPage := messagePageOf(uri)

	port, err := h.forge.For(ctx)
	if err != nil {
		return nil, fmt.Errorf("forgejo client: %w", err)
	}

	commit, err := apprepo.NewGitService(repogitadapter.New(port)).ReadCommit(ctx, params.Owner, params.Repo, params.SHA)
	if err != nil {
		return nil, resource.MapForgejoError(uri, err)
	}

	part := commitMessagePart(commit, msgPage)
	nextFiles := commitReadURI(params.Owner, params.Repo, params.SHA, list.Paging{Page: pg.Page + 1, Limit: pg.Limit}, msgPage)
	nextMsg := commitReadURI(params.Owner, params.Repo, params.SHA, pg, part.page+1)
	jsonBytes, err := marshalBoundedCommit(commit, pg, part, nextFiles, nextMsg)
	if err != nil {
		return nil, fmt.Errorf("marshal commit: %w", err)
	}

	contents := []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      uri,
			MIMEType: "application/json",
			Text:     string(jsonBytes),
		},
	}

	if part.text != "" || part.cut {
		message := part.text
		if part.cut {
			message += commitMessageMarker(part)
		}
		contents = append(contents, mcp.TextResourceContents{
			URI:      uri,
			MIMEType: "text/markdown",
			Text:     message,
		})
	}

	return contents, nil
}
