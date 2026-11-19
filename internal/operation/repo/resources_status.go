// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"
	"encoding/json"
	"fmt"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/model"
	apprepo "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/repo"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge/repoadminadapter"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/log"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/resource"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const statusResourceURITemplate = "forgejo://repo/{owner}/{repo}/commit/{sha}/status"

// statusResourcePayload is the JSON body returned as the primary content block.
type statusResourcePayload struct {
	SHA   string `json:"sha"`
	State string `json:"state"`
	// TotalCount is Forgejo's grand total of statuses on the commit.
	TotalCount int                `json:"total_count"`
	Statuses   []model.StatusItem `json:"statuses"`
	// Truncated is true when the statuses list is cut at the embedded cap, or when
	// the commit is too large to read in full, in which case State is unknown.
	// ListTool then names get_commit_statuses, which pages the full list.
	Truncated bool   `json:"truncated,omitempty"`
	ListTool  string `json:"list_tool,omitempty"`
}

// RegisterStatusResource registers the forgejo://repo/{owner}/{repo}/commit/{sha}/status
// resource template. The resource aggregates per-context CI statuses for a commit SHA and
// returns a combined state. This is the canonical "is the commit green?" surface. The
// combined state is computed server-side from the per-context list. The resource is
// pinned to an immutable SHA; clients may cache responses keyed by URI.
func RegisterStatusResource(s *server.MCPServer, f forge.Factory) {
	h := handlers{forge: f}
	resource.RegisterTemplate(
		s,
		statusResourceURITemplate,
		"Forgejo Commit Status",
		h.statusResourceHandler,
		mcp.WithTemplateDescription(
			"Aggregated CI status for a Forgejo repository commit. "+
				"URI: forgejo://repo/{owner}/{repo}/commit/{sha}/status — sha must be a full 40-character hex SHA. "+
				"Returns combined state (success|failure|pending|unknown) plus bounded per-context statuses. "+
				"Pinned to an immutable SHA; safe to cache by URI. "+
				"Use get_commit_statuses tool for paginated per-context enumeration.",
		),
		mcp.WithTemplateMIMEType("application/json"),
	)
	log.Debug("Registered commit status resource template")
}

func (h handlers) statusResourceHandler(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := req.Params.URI
	params, err := resource.ParseStatus(uri)
	if err != nil {
		return nil, resource.MapForgejoError(uri, err)
	}

	port, err := h.forge.For(ctx)
	if err != nil {
		return nil, fmt.Errorf("forgejo client: %w", err)
	}

	// Every status of the commit is read, so the state covers all of them. Only
	// the list in the payload is bounded; a commit too large to read in full
	// reports an unknown state, flagged truncated.
	sum, err := apprepo.NewAdmin(repoadminadapter.New(port)).CommitStatusSummary(ctx, params.Owner, params.Repo, params.SHA)
	if err != nil {
		return nil, resource.MapForgejoError(uri, err)
	}
	statuses := sum.Statuses

	items := make([]string, 0, len(statuses))
	for _, s := range statuses {
		items = append(items, s.State)
	}

	bounded := resource.Bounded(items, resource.EmbeddedListCap, "get_commit_statuses")

	boundedItems := statuses
	if bounded.Truncated {
		boundedItems = statuses[:resource.EmbeddedListCap]
	}

	truncated := bounded.Truncated || !sum.Complete
	payload := statusResourcePayload{
		SHA:        params.SHA,
		State:      sum.State,
		TotalCount: sum.TotalCount,
		Statuses:   boundedItems,
		Truncated:  truncated,
	}
	if truncated {
		payload.ListTool = resource.ToolRef("get_commit_statuses")
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal status payload: %w", err)
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      uri,
			MIMEType: "application/json",
			Text:     string(jsonBytes),
		},
	}, nil
}
