// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/list"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	apprepo "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/repo"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/repogitadapter"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/log"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/resource"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// commitResourceURITemplate carries the {?page,limit} expansion, which is
// load-bearing: a template registered bare matches only the bare URI, so a
// query-bearing read would fail with "resource not found" (see the labels
// template in internal/operation/issue/resources_label.go).
const commitResourceURITemplate = "forgejo://repo/{owner}/{repo}/commit/{sha}{?page,limit}"

// RegisterCommitResource registers the forgejo://repo/{owner}/{repo}/commit/{sha}{?page,limit} resource template.
func RegisterCommitResource(s *server.MCPServer, f forge.Factory) {
	h := handlers{forge: f}
	resource.RegisterTemplate(
		s,
		commitResourceURITemplate,
		"Forgejo Commit",
		h.commitResourceHandler,
		mcp.WithTemplateDescription(
			"Immutable commit metadata for a Forgejo repository commit. "+
				"URI: forgejo://repo/{owner}/{repo}/commit/{sha}{?page,limit} — sha must be a full 40-character hex SHA. "+
				"Returns JSON metadata plus a text/markdown body sidecar for the commit message. "+
				"The file list is one page at a time: page and limit (default and maximum 30 files) select it, and files_next names the next page when more follow.",
		),
		mcp.WithTemplateMIMEType("application/json"),
	)
	log.Debug("Registered commit resource template")
}

// commitMessageCap bounds the commit message in both content blocks, so a
// pathological message cannot fill the context. A cut message is marked, not
// refetched: the sub-rule 1 truncation marker is the decision for this bound
// (docs/design/output-bounding.md).
const commitMessageCap = 16 * 1024

// commitMessageMarker is what the markdown sidecar ends with when the message
// was cut.
const commitMessageMarker = "\n\n[message truncated: over 16384 bytes]"

// marshalBoundedCommit encodes a copy of c with its message capped at
// commitMessageCap and its file list cut to the page pg names. The commit's
// own keys come first. When the message is cut, message_truncated is added;
// when the file list is cut, files_total, files_page, files_limit and
// files_truncated are added, and files_next names the next page when one
// follows. A commit that fits in its page encodes byte-for-byte as before.
func marshalBoundedCommit(c *model.Commit, pg list.Paging, next string) ([]byte, error) {
	cp := *c
	var extra []string
	if cp.RepoCommit != nil && len(cp.RepoCommit.Message) > commitMessageCap {
		rc := *cp.RepoCommit
		rc.Message = resource.Excerpt(rc.Message, commitMessageCap)
		cp.RepoCommit = &rc
		extra = append(extra, `"message_truncated":true`)
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
			nb, err := json.Marshal(next)
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

// commitNextURI is the URI of the file page after pg for one commit.
func commitNextURI(owner, repo, sha string, pg list.Paging) string {
	return fmt.Sprintf("forgejo://repo/%s/%s/commit/%s?page=%d&limit=%d",
		url.PathEscape(owner), url.PathEscape(repo), url.PathEscape(sha), pg.Page+1, pg.Limit)
}

func (h handlers) commitResourceHandler(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := req.Params.URI
	params, err := resource.ParseCommit(uri)
	if err != nil {
		return nil, resource.MapForgejoError(uri, err)
	}
	page, limit := resource.PageLimit(req)
	pg := list.Paging{Page: page, Limit: limit}

	port, err := h.forge.For(ctx)
	if err != nil {
		return nil, fmt.Errorf("forgejo client: %w", err)
	}

	commit, err := apprepo.NewGitService(repogitadapter.New(port)).ReadCommit(ctx, params.Owner, params.Repo, params.SHA)
	if err != nil {
		return nil, resource.MapForgejoError(uri, err)
	}

	jsonBytes, err := marshalBoundedCommit(commit, pg, commitNextURI(params.Owner, params.Repo, params.SHA, pg))
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

	if commit.RepoCommit != nil && commit.RepoCommit.Message != "" {
		message := resource.Excerpt(commit.RepoCommit.Message, commitMessageCap)
		if len(commit.RepoCommit.Message) > commitMessageCap {
			message += commitMessageMarker
		}
		contents = append(contents, mcp.TextResourceContents{
			URI:      uri,
			MIMEType: "text/markdown",
			Text:     message,
		})
	}

	return contents, nil
}
