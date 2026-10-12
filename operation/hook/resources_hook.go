// SPDX-License-Identifier: GPL-3.0-or-later

package hook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	apphook "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/hook"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/resource"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/hookadapter"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"

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
	Sentinel  string        `json:"sentinel,omitempty"`
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
	if errors.Is(err, apphook.ErrRequired) {
		return resource.MapForgejoError(uri, fmt.Errorf("%w: %s", resource.ErrInvalidParams, err))
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

	col, err := apphook.New(hookadapter.New(port)).ReadCollection(ctx, p.Owner, p.Repo, resource.EmbeddedListCap)
	if err != nil {
		return nil, mapErr(uri, err)
	}

	payload := hooksCollectionPayload{
		Owner:     p.Owner,
		Repo:      p.Repo,
		Count:     len(col.Hooks),
		Hooks:     col.Hooks,
		Truncated: col.Truncated,
	}
	if col.Truncated {
		// The note names the list tool only when the registry has it.
		bounded := resource.BoundedResult{Cap: resource.EmbeddedListCap, ListTool: ListRepoHooksToolName, Truncated: true}
		payload.Sentinel = bounded.Sentinel()
		payload.ListTool = resource.ToolRef(ListRepoHooksToolName)
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

	hk, err := apphook.New(hookadapter.New(port)).Get(ctx, apphook.HookRef{Owner: p.Owner, Repo: p.Repo, ID: float64(p.ID)})
	if err != nil {
		return nil, mapErr(uri, err)
	}

	jsonBytes, err := json.Marshal(hk)
	if err != nil {
		return nil, fmt.Errorf("marshal hook payload: %w", err)
	}
	return []mcp.ResourceContents{
		mcp.TextResourceContents{URI: uri, MIMEType: "application/json", Text: string(jsonBytes)},
	}, nil
}
