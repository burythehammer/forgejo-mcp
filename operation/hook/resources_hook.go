// SPDX-License-Identifier: GPL-3.0-or-later

package hook

import (
	"context"
	"encoding/json"
	"fmt"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/resource"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	hooksResourceURITemplate = "forgejo://repo/{owner}/{repo}/hooks"
	hookResourceURITemplate  = "forgejo://repo/{owner}/{repo}/hook/{id}"
)

// hooksCollectionPayload is the JSON body for the hooks collection resource.
type hooksCollectionPayload struct {
	Owner     string        `json:"owner"`
	Repo      string        `json:"repo"`
	Count     int           `json:"count"`
	Hooks     []hookPayload `json:"hooks"`
	Truncated bool          `json:"truncated,omitempty"`
	ListTool  string        `json:"list_tool,omitempty"`
}

// RegisterResources registers the webhook collection and single-hook
// resource templates on the MCP server.
func RegisterResources(s *server.MCPServer, f forge.Factory) {
	h := handlers{forge: f}
	resource.RegisterTemplate(
		s,
		hooksResourceURITemplate,
		"Forgejo Repository Webhooks",
		h.hooksResourceHandler,
		mcp.WithTemplateDescription(
			"Webhooks for a Forgejo repository. "+
				"URI: forgejo://repo/{owner}/{repo}/hooks. "+
				"Returns the hooks as a read-only JSON document, bounded to the first "+
				"EmbeddedListCap hooks; when truncated, use the list_repo_hooks "+
				"tool to page through the remainder. Secret fields are never returned.",
		),
		mcp.WithTemplateMIMEType("application/json"),
	)
	resource.RegisterTemplate(
		s,
		hookResourceURITemplate,
		"Forgejo Repository Webhook",
		h.hookResourceHandler,
		mcp.WithTemplateDescription(
			"A single repository webhook for a Forgejo repository. "+
				"URI: forgejo://repo/{owner}/{repo}/hook/{id} — id is the numeric webhook ID. "+
				"Returns the hook as a read-only JSON document. Secret fields are never returned.",
		),
		mcp.WithTemplateMIMEType("application/json"),
	)
	log.Debug("Registered hook resource templates")
}

// mapErr maps a port failure to a resource error, keeping the HTTP status in
// the message so the mapping can tell forbidden from not found.
func mapErr(uri string, err error) error {
	if code := forge.StatusCode(err); code != 0 {
		return resource.MapForgejoError(uri, fmt.Errorf("%d %s", code, err.Error()))
	}
	return resource.MapForgejoError(uri, err)
}

func (h handlers) hooksResourceHandler(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := req.Params.URI
	p, err := resource.ParseHooks(uri)
	if err != nil {
		return nil, resource.MapForgejoError(uri, err)
	}

	port, err := h.forge.For(ctx)
	if err != nil {
		return nil, fmt.Errorf("forgejo client: %w", err)
	}

	// Request EmbeddedListCap+1 so Bounded can distinguish "at cap" from "over cap".
	list, err := port.ListRepoHooks(ctx, p.Owner, p.Repo, forgejo_sdk.ListHooksOptions{
		ListOptions: forgejo_sdk.ListOptions{PageSize: resource.EmbeddedListCap + 1},
	})
	if err != nil {
		return nil, mapErr(uri, err)
	}
	hooks := list.Hooks

	ids := make([]string, len(hooks))
	for i, h := range hooks {
		ids[i] = fmt.Sprintf("%d", h.ID)
	}
	bounded := resource.Bounded(ids, resource.EmbeddedListCap, "list_repo_hooks")

	shown := hooks
	if bounded.Truncated {
		shown = hooks[:resource.EmbeddedListCap]
	}

	payloads := make([]hookPayload, len(shown))
	for i, h := range shown {
		payloads[i] = safeHook(h)
	}

	payload := hooksCollectionPayload{
		Owner:     p.Owner,
		Repo:      p.Repo,
		Count:     len(payloads),
		Hooks:     payloads,
		Truncated: bounded.Truncated,
	}
	if bounded.Truncated {
		payload.ListTool = "list_repo_hooks"
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal hooks payload: %w", err)
	}
	return []mcp.ResourceContents{
		mcp.TextResourceContents{URI: uri, MIMEType: "application/json", Text: string(jsonBytes)},
	}, nil
}

func (h handlers) hookResourceHandler(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := req.Params.URI
	p, err := resource.ParseHook(uri)
	if err != nil {
		return nil, resource.MapForgejoError(uri, err)
	}

	port, err := h.forge.For(ctx)
	if err != nil {
		return nil, fmt.Errorf("forgejo client: %w", err)
	}

	hk, err := port.GetRepoHook(ctx, p.Owner, p.Repo, p.ID)
	if err != nil {
		return nil, mapErr(uri, err)
	}

	jsonBytes, err := json.Marshal(safeHook(hk))
	if err != nil {
		return nil, fmt.Errorf("marshal hook payload: %w", err)
	}
	return []mcp.ResourceContents{
		mcp.TextResourceContents{URI: uri, MIMEType: "application/json", Text: string(jsonBytes)},
	}, nil
}
