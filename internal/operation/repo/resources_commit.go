// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"
	"encoding/json"
	"fmt"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	apprepo "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/repo"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/repogitadapter"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/log"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/resource"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const commitResourceURITemplate = "forgejo://repo/{owner}/{repo}/commit/{sha}"

// RegisterCommitResource registers the forgejo://repo/{owner}/{repo}/commit/{sha} resource template.
func RegisterCommitResource(s *server.MCPServer, f forge.Factory) {
	h := handlers{forge: f}
	resource.RegisterTemplate(
		s,
		commitResourceURITemplate,
		"Forgejo Commit",
		h.commitResourceHandler,
		mcp.WithTemplateDescription(
			"Immutable commit metadata for a Forgejo repository commit. "+
				"URI: forgejo://repo/{owner}/{repo}/commit/{sha} — sha must be a full 40-character hex SHA. "+
				"Returns JSON metadata plus a text/markdown body sidecar for the commit message.",
		),
		mcp.WithTemplateMIMEType("application/json"),
	)
	log.Debug("Registered commit resource template")
}

// commitMessageCap bounds the commit message in both content blocks, so a
// pathological message cannot fill the context (docs/design/output-bounding.md).
const commitMessageCap = 16 * 1024

// marshalBoundedCommit encodes a copy of c with its message capped at
// commitMessageCap and its file list capped at resource.EmbeddedListCap. When
// the list is cut, files_total and files_truncated are appended after the
// commit's own keys, so a commit that fits encodes byte-for-byte as before.
func marshalBoundedCommit(c *model.Commit) ([]byte, error) {
	cp := *c
	if cp.RepoCommit != nil {
		rc := *cp.RepoCommit
		rc.Message = resource.Excerpt(rc.Message, commitMessageCap)
		cp.RepoCommit = &rc
	}
	truncated := len(cp.Files) > resource.EmbeddedListCap
	total := len(cp.Files)
	if truncated {
		cp.Files = cp.Files[:resource.EmbeddedListCap]
	}
	b, err := json.Marshal(&cp)
	if err != nil || !truncated {
		return b, err
	}
	return append(b[:len(b)-1], fmt.Sprintf(`,"files_total":%d,"files_truncated":true}`, total)...), nil
}

func (h handlers) commitResourceHandler(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := req.Params.URI
	params, err := resource.ParseCommit(uri)
	if err != nil {
		return nil, resource.MapForgejoError(uri, err)
	}

	port, err := h.forge.For(ctx)
	if err != nil {
		return nil, fmt.Errorf("forgejo client: %w", err)
	}

	commit, err := apprepo.NewGitService(repogitadapter.New(port)).ReadCommit(ctx, params.Owner, params.Repo, params.SHA)
	if err != nil {
		return nil, resource.MapForgejoError(uri, err)
	}

	jsonBytes, err := marshalBoundedCommit(commit)
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
		contents = append(contents, mcp.TextResourceContents{
			URI:      uri,
			MIMEType: "text/markdown",
			Text:     message,
		})
	}

	return contents, nil
}
