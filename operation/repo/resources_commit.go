// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"
	"encoding/json"
	"fmt"

	apprepo "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/repo"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/resource"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/repogitadapter"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"

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
		if code := forge.StatusCode(err); code != 0 {
			return nil, resource.MapForgejoError(uri, fmt.Errorf("%d %s", code, err.Error()))
		}
		return nil, resource.MapForgejoError(uri, err)
	}

	jsonBytes, err := json.Marshal(commit)
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
		contents = append(contents, mcp.TextResourceContents{
			URI:      uri,
			MIMEType: "text/markdown",
			Text:     commit.RepoCommit.Message,
		})
	}

	return contents, nil
}
