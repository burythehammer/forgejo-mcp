// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"
	"encoding/json"
	"fmt"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	apprepo "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/repo"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/repoadminadapter"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/log"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/resource"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const statusResourceURITemplate = "forgejo://repo/{owner}/{repo}/commit/{sha}/status"

// statusResourcePayload is the JSON body returned as the primary content block.
type statusResourcePayload struct {
	SHA   string `json:"sha"`
	State string `json:"state"`
	// TotalCount is the number of statuses the read fetched (at most the
	// embedded cap plus one), not Forgejo's grand total. It is kept as it was
	// shipped; get_commit_statuses carries the grand total.
	TotalCount int                `json:"total_count"`
	Statuses   []model.StatusItem `json:"statuses"`
	Truncated  bool               `json:"truncated,omitempty"`
	ListTool   string             `json:"list_tool,omitempty"`
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

	// The first page, one row over the embedded cap, so that resource.Bounded can
	// tell "exactly at cap" from "over cap". The read computes the aggregate
	// state over the rows it fetched.
	read, err := apprepo.NewAdmin(repoadminadapter.New(port)).ReadCommitStatuses(ctx, params.Owner, params.Repo, params.SHA, 1, resource.EmbeddedListCap+1)
	if err != nil {
		return nil, resource.MapForgejoError(uri, err)
	}
	statuses := read.Statuses

	items := make([]string, 0, len(statuses))
	for _, s := range statuses {
		items = append(items, s.State)
	}

	bounded := resource.Bounded(items, resource.EmbeddedListCap, "get_commit_statuses")

	boundedItems := statuses
	if bounded.Truncated {
		boundedItems = statuses[:resource.EmbeddedListCap]
	}

	payload := statusResourcePayload{
		SHA:        params.SHA,
		State:      read.State,
		TotalCount: len(statuses),
		Statuses:   boundedItems,
		Truncated:  bounded.Truncated,
	}
	if bounded.Truncated {
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
