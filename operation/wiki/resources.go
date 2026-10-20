// SPDX-License-Identifier: GPL-3.0-or-later

package wiki

import (
	"context"
	"encoding/json"
	"fmt"
	"unicode/utf8"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/model"
	appwiki "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/wiki"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/operation/resource"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge/wikiadapter"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/log"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterWikiResource registers the wiki page resource template, served from
// the factory like the tools.
func RegisterWikiResource(s *server.MCPServer, f forge.Factory) {
	h := handlers{forge: f}
	resource.RegisterTemplate(s,
		"forgejo://repo/{owner}/{repo}/wiki/{pageName}",
		"Forgejo Wiki Page",
		h.wikiResourceHandler,
		mcp.WithTemplateDescription("Single wiki page metadata plus a text/markdown sidecar. URI: forgejo://repo/{owner}/{repo}/wiki/{pageName}. Use the server-normalized page name and percent-encode characters such as slash (%2F) and space (%20), for example Guides%2FGetting%20Started. If slash-bearing names cannot resolve through the resource, use get_wiki_page."),
		mcp.WithTemplateMIMEType("application/json"),
	)
	log.Debug("Registered wiki resource template")
}

// wikiResourcePayload is the JSON body of the wiki page resource.
type wikiResourcePayload struct {
	Owner           string                      `json:"owner"`
	Repo            string                      `json:"repo"`
	PageName        string                      `json:"page_name"`
	Title           string                      `json:"title"`
	CommitSHA       string                      `json:"commit_sha"`
	RecentRevisions []model.WikiRevisionSummary `json:"recent_revisions"`
	Truncated       bool                        `json:"truncated,omitempty"`
	RevisionCount   int                         `json:"revision_count,omitempty"`
	ListTool        string                      `json:"list_tool,omitempty"`
	Sentinel        string                      `json:"sentinel,omitempty"`
}

func (h handlers) wikiResourceHandler(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := req.Params.URI
	p, err := resource.ParseWiki(uri)
	if err != nil {
		return nil, resource.MapForgejoError(uri, err)
	}
	f, err := h.forge.For(ctx)
	if err != nil {
		return nil, resource.MapForgejoError(uri, err)
	}
	read, err := appwiki.New(wikiadapter.New(f)).ReadPage(ctx, p.Owner, p.Repo, p.PageName, resource.EmbeddedListCap)
	if err != nil {
		return nil, resource.MapForgejoError(uri, err)
	}

	payload := wikiResourcePayload{
		Owner:           p.Owner,
		Repo:            p.Repo,
		PageName:        read.PageName,
		Title:           read.Title,
		CommitSHA:       read.CommitSHA,
		RecentRevisions: read.Revisions,
		Truncated:       read.RevisionsTruncated,
	}
	if read.RevisionsTruncated {
		// The server said more revisions exist: WithMoreRemaining takes its total
		// as the server's count, and Sentinel names the tool only when registered.
		bounded := resource.BoundedResult{Cap: resource.EmbeddedListCap, ListTool: GetWikiRevisionsToolName}.WithMoreRemaining(read.RevisionCount)
		payload.RevisionCount = read.RevisionCount
		payload.ListTool = resource.ToolRef(GetWikiRevisionsToolName)
		payload.Sentinel = bounded.Sentinel()
	}
	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal wiki payload: %w", err)
	}

	markdown := read.Content
	if len(markdown) > forge.MaxInlineDownloadBytes {
		marker := wikiTruncationMarker()
		keep := forge.MaxInlineDownloadBytes - len(marker)
		for keep > 0 && !utf8.RuneStart(markdown[keep]) {
			keep--
		}
		markdown = markdown[:keep] + marker
	}
	return []mcp.ResourceContents{
		mcp.TextResourceContents{URI: uri, MIMEType: "application/json", Text: string(jsonBytes)},
		mcp.TextResourceContents{URI: uri, MIMEType: "text/markdown", Text: markdown},
	}, nil
}

// wikiTruncationMarker is the note appended to inline markdown that exceeds the
// download limit. The tool it names comes from the registry, so it is omitted
// rather than named when that tool is not registered.
func wikiTruncationMarker() string {
	if ref := resource.ToolRef(GetWikiPageToolName); ref != "" {
		return "\n\n[truncated: use " + ref + " with start_line/end_line to retrieve the remainder.]"
	}
	return "\n\n[truncated: use start_line/end_line to retrieve the remainder.]"
}
