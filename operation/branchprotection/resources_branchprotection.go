// SPDX-License-Identifier: GPL-3.0-or-later

package branchprotection

import (
	"context"
	"encoding/json"
	"fmt"

	appbp "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/branchprotection"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/model"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/operation/resource"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge/branchprotectionadapter"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	branchProtectionsResourceURITemplate = "forgejo://repo/{owner}/{repo}/branch_protections"
	branchProtectionResourceURITemplate  = "forgejo://repo/{owner}/{repo}/branch_protection/{rule}"
)

// bpCollectionPayload is the JSON body for the branch_protections collection resource.
type bpCollectionPayload struct {
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
	// TotalCount is the number of rules embedded here, at most the cap. The
	// listing does not report a repository total, so Truncated says whether
	// there are more.
	TotalCount        int                       `json:"total_count"`
	BranchProtections []*model.BranchProtection `json:"branch_protections"`
	Truncated         bool                      `json:"truncated,omitempty"`
	ListTool          string                    `json:"list_tool,omitempty"`
	Sentinel          string                    `json:"sentinel,omitempty"`
}

// RegisterResource registers the branch protection collection and single-rule
// resource templates on the MCP server.
func RegisterResource(s *server.MCPServer, f forge.Factory) {
	h := handlers{forge: f}
	resource.RegisterTemplate(
		s,
		branchProtectionsResourceURITemplate,
		"Forgejo Branch Protections",
		h.branchProtectionsResourceHandler,
		mcp.WithTemplateDescription(
			"Branch protection rules for a Forgejo repository. "+
				"URI: forgejo://repo/{owner}/{repo}/branch_protections. "+
				"Returns the rules as a read-only JSON document, bounded to the first "+
				"EmbeddedListCap rules; when truncated, use the list_branch_protections "+
				"tool to page through the remainder.",
		),
		mcp.WithTemplateMIMEType("application/json"),
	)
	resource.RegisterTemplate(
		s,
		branchProtectionResourceURITemplate,
		"Forgejo Branch Protection",
		h.branchProtectionResourceHandler,
		mcp.WithTemplateDescription(
			"A single branch protection rule for a Forgejo repository. "+
				"URI: forgejo://repo/{owner}/{repo}/branch_protection/{rule} — rule is the rule name. "+
				"Rule names are branch patterns and commonly contain slashes: encode a "+
				"literal '/' as %2F and spaces as %20, without double-encoding, e.g. "+
				"release%2Fv1. A raw '/' does not resolve, and %252F resolves to the "+
				"literal text %2F rather than a slash. "+
				"Returns the rule's full protection state as a read-only JSON document.",
		),
		mcp.WithTemplateMIMEType("application/json"),
	)
	log.Debug("Registered branch protection resource templates")
}

// mapErr maps a port failure to a resource error, keeping the HTTP status in
// the message so the mapping can tell forbidden from not found.
func mapErr(uri string, err error) error {
	return resource.MapForgejoError(uri, err)
}

func (h handlers) branchProtectionsResourceHandler(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := req.Params.URI
	p, err := resource.ParseBranchProtections(uri)
	if err != nil {
		return nil, resource.MapForgejoError(uri, err)
	}

	port, err := h.forge.For(ctx)
	if err != nil {
		return nil, fmt.Errorf("forgejo client: %w", err)
	}

	col, err := appbp.New(branchprotectionadapter.New(port)).ReadCollection(ctx, p.Owner, p.Repo, resource.EmbeddedListCap)
	if err != nil {
		return nil, mapErr(uri, err)
	}

	payload := bpCollectionPayload{
		Owner:             p.Owner,
		Repo:              p.Repo,
		TotalCount:        len(col.Rules),
		BranchProtections: col.Rules,
		Truncated:         col.Truncated,
	}
	if col.Truncated {
		// The note names the list tool only when the registry has it.
		bounded := resource.BoundedResult{Cap: resource.EmbeddedListCap, ListTool: ListBranchProtectionsToolName, Truncated: true}
		payload.Sentinel = bounded.Sentinel()
		payload.ListTool = resource.ToolRef(ListBranchProtectionsToolName)
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal branch protections payload: %w", err)
	}
	return []mcp.ResourceContents{
		mcp.TextResourceContents{URI: uri, MIMEType: "application/json", Text: string(jsonBytes)},
	}, nil
}

func (h handlers) branchProtectionResourceHandler(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := req.Params.URI
	p, err := resource.ParseBranchProtection(uri)
	if err != nil {
		return nil, resource.MapForgejoError(uri, err)
	}

	port, err := h.forge.For(ctx)
	if err != nil {
		return nil, fmt.Errorf("forgejo client: %w", err)
	}

	bp, err := appbp.New(branchprotectionadapter.New(port)).Get(ctx, appbp.RuleRef{Owner: p.Owner, Repo: p.Repo, Rule: p.Rule})
	if err != nil {
		return nil, mapErr(uri, err)
	}

	jsonBytes, err := json.Marshal(bp)
	if err != nil {
		return nil, fmt.Errorf("marshal branch protection payload: %w", err)
	}
	return []mcp.ResourceContents{
		mcp.TextResourceContents{URI: uri, MIMEType: "application/json", Text: string(jsonBytes)},
	}, nil
}
