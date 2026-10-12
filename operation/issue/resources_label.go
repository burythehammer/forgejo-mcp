// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	apppissue "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/issue"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/resource"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/issueadapter"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterLabelResources registers the label resource templates.
func RegisterLabelResources(s *server.MCPServer, f forge.Factory) {
	h := newHandlers(f)
	resource.RegisterTemplate(
		s,
		"forgejo://repo/{owner}/{repo}/label/{id}",
		"Forgejo Repo Label",
		h.repoLabelResourceHandler,
		mcp.WithTemplateDescription(
			"Single repository label by id. "+
				"URI: forgejo://repo/{owner}/{repo}/label/{id}.",
		),
		mcp.WithTemplateMIMEType("application/json"),
	)
	// The {?…} expansion is load-bearing, not documentation: mcp-go matches a
	// read against an anchored regexp built from this exact string, so a
	// template registered bare matches only the bare URI and every
	// query-bearing read fails with "resource not found" before any handler
	// runs. Registered bare, this resource's page/limit were unreachable —
	// and so, in turn, was the paging defect below.
	resource.RegisterTemplate(
		s,
		"forgejo://repo/{owner}/{repo}/labels{?page,limit}",
		"Forgejo Repo Labels",
		h.repoLabelsResourceHandler,
		mcp.WithTemplateDescription(
			"Bounded list of repository labels (page/limit client-controlled, ceiling "+
				strconv.Itoa(resource.EmbeddedListCap)+"). "+
				"URI: forgejo://repo/{owner}/{repo}/labels{?page,limit}. "+
				"Use list_repo_labels tool for the unbounded enumeration path.",
		),
		mcp.WithTemplateMIMEType("application/json"),
	)
	resource.RegisterTemplate(
		s,
		// See the note on the repo-labels template above: the {?…} expansion
		// is what makes page/limit reachable at all.
		"forgejo://org/{org}/labels{?page,limit}",
		"Forgejo Org Labels",
		h.orgLabelsResourceHandler,
		mcp.WithTemplateDescription(
			"Bounded list of organization-level labels (page/limit client-controlled, ceiling "+
				strconv.Itoa(resource.EmbeddedListCap)+"). "+
				"URI: forgejo://org/{org}/labels{?page,limit}. "+
				"Use list_org_labels tool for the unbounded enumeration path.",
		),
		mcp.WithTemplateMIMEType("application/json"),
	)
	log.Debug("Registered label resource templates")
}

// ---- single repo label ----

type labelResourcePayload struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
	URL         string `json:"url"`
}

// labelPayloadOf is the resource's flattened encoding of one label.
func labelPayloadOf(l model.Label) labelResourcePayload {
	return labelResourcePayload{ID: l.ID, Name: l.Name, Color: l.Color, Description: l.Description, URL: l.URL}
}

func (h handlers) repoLabelResourceHandler(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := req.Params.URI
	p, err := resource.ParseLabel(uri)
	if err != nil {
		return nil, resource.MapForgejoError(uri, err)
	}

	fg, err := h.forge.For(ctx)
	if err != nil {
		return nil, fmt.Errorf("forgejo client: %w", err)
	}

	label, err := apppissue.New(issueadapter.New(fg)).GetRepoLabel(ctx, p.Owner, p.Repo, p.ID)
	if err != nil {
		return nil, mapErr(uri, err)
	}

	jsonBytes, err := json.Marshal(labelPayloadOf(*label))
	if err != nil {
		return nil, fmt.Errorf("marshal label payload: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{URI: uri, MIMEType: "application/json", Text: string(jsonBytes)},
	}, nil
}

// ---- repo labels list ----

type labelsListPayload struct {
	Owner     string                 `json:"owner"`
	Repo      string                 `json:"repo"`
	Labels    []labelResourcePayload `json:"labels"`
	Truncated bool                   `json:"truncated,omitempty"`
	ListTool  string                 `json:"list_tool,omitempty"`
	Sentinel  string                 `json:"sentinel,omitempty"`
}

func (h handlers) repoLabelsResourceHandler(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := req.Params.URI
	p, err := resource.ParseLabels(uri)
	if err != nil {
		return nil, resource.MapForgejoError(uri, err)
	}

	page, limit := pageLimit(req)

	fg, err := h.forge.For(ctx)
	if err != nil {
		return nil, fmt.Errorf("forgejo client: %w", err)
	}

	// The read asks for exactly limit rows and takes "more exists" from the
	// server, never from a limit+1 probe: see the paging note on
	// app/issue.ReadRepoLabels. The organization's labels are not part of
	// this resource; list_repo_labels merges them by default.
	ad := issueadapter.New(fg)
	limit = clampToCeiling(ctx, ad, limit)
	list, err := apppissue.New(ad).ReadRepoLabels(ctx, apppissue.RepoLabelsInput{
		Owner: p.Owner, Repo: p.Repo, Paging: apppissue.Paging{Page: page, Limit: limit},
	})
	if err != nil {
		return nil, mapErr(uri, err)
	}

	bounded := resource.Bounded(labelIDs(list.Labels), limit, ListRepoLabelsToolName)
	if list.More {
		bounded = bounded.WithMoreRemaining(knownTotal(list.Total))
	}

	labels := make([]labelResourcePayload, 0, len(bounded.Items))
	for _, l := range list.Labels[:len(bounded.Items)] {
		labels = append(labels, labelPayloadOf(l.Label))
	}

	payload := labelsListPayload{
		Owner:     p.Owner,
		Repo:      p.Repo,
		Labels:    labels,
		Truncated: bounded.Truncated,
	}
	if bounded.Truncated {
		payload.ListTool = ListRepoLabelsToolName
		payload.Sentinel = bounded.Sentinel()
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal labels payload: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{URI: uri, MIMEType: "application/json", Text: string(jsonBytes)},
	}, nil
}

// ---- org labels list ----

type orgLabelsListPayload struct {
	Org       string                 `json:"org"`
	Labels    []labelResourcePayload `json:"labels"`
	Truncated bool                   `json:"truncated,omitempty"`
	ListTool  string                 `json:"list_tool,omitempty"`
	Sentinel  string                 `json:"sentinel,omitempty"`
}

func (h handlers) orgLabelsResourceHandler(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := req.Params.URI
	p, err := resource.ParseOrgLabels(uri)
	if err != nil {
		return nil, resource.MapForgejoError(uri, err)
	}

	page, limit := pageLimit(req)

	fg, err := h.forge.For(ctx)
	if err != nil {
		return nil, fmt.Errorf("forgejo client: %w", err)
	}
	ad := issueadapter.New(fg)
	limit = clampToCeiling(ctx, ad, limit)
	list, err := apppissue.New(ad).ReadOrgLabels(ctx, p.Org, apppissue.Paging{Page: page, Limit: limit})
	if err != nil {
		return nil, mapErr(uri, err)
	}

	bounded := resource.Bounded(labelIDs(list.Labels), limit, ListOrgLabelsToolName)
	if list.More {
		bounded = bounded.WithMoreRemaining(knownTotal(list.Total))
	}

	labels := make([]labelResourcePayload, 0, len(bounded.Items))
	for _, l := range list.Labels[:len(bounded.Items)] {
		labels = append(labels, labelPayloadOf(l.Label))
	}

	payload := orgLabelsListPayload{
		Org:       p.Org,
		Labels:    labels,
		Truncated: bounded.Truncated,
	}
	if bounded.Truncated {
		payload.ListTool = ListOrgLabelsToolName
		payload.Sentinel = bounded.Sentinel()
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal org labels payload: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{URI: uri, MIMEType: "application/json", Text: string(jsonBytes)},
	}, nil
}

// labelIDs names each label by its ID, the items the bound counts.
func labelIDs(labels []model.ScopedLabel) []string {
	out := make([]string, len(labels))
	for i, l := range labels {
		out[i] = strconv.FormatInt(l.ID, 10)
	}
	return out
}

// knownTotal is the server's total when it reported one, and zero otherwise,
// which WithMoreRemaining reads as "total unknown".
func knownTotal(t apppissue.Total) int {
	if !t.Known {
		return 0
	}
	return t.Count
}

// clampToCeiling lowers a resource's limit to the instance ceiling when one is
// known. A resource embeds a page rather than refusing a limit, so it serves
// what the instance allows, as it did before the list tools applied the
// ceiling.
func clampToCeiling(ctx context.Context, src apppissue.Port, limit int) int {
	if max, ok := src.MaxResponseItems(ctx); ok && max >= 1 && limit > max {
		return max
	}
	return limit
}

// pageLimit extracts page and limit from resource URI query params with
// EmbeddedListCap as the ceiling for limit. Malformed or absent values
// fall back to page=1, limit=EmbeddedListCap.
func pageLimit(req mcp.ReadResourceRequest) (page, limit int) {
	page = 1
	limit = resource.EmbeddedListCap
	u, err := url.Parse(req.Params.URI)
	if err != nil {
		return page, limit
	}
	q := u.Query()
	if v, err := strconv.Atoi(q.Get("page")); err == nil && v >= 1 {
		page = v
	}
	if v, err := strconv.Atoi(q.Get("limit")); err == nil && v >= 1 {
		limit = v
		if limit > resource.EmbeddedListCap {
			limit = resource.EmbeddedListCap
		}
	}
	return page, limit
}
