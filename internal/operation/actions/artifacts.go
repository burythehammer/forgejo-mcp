// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"math"

	app "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/actions"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/log"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/decode"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/params"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/to"

	"github.com/mark3labs/mcp-go/mcp"
)

// artifactsArgs are the arguments of list_action_run_artifacts.
type artifactsArgs struct {
	Owner string `json:"owner" required:"true"`
	Repo  string `json:"repo" required:"true"`
	RunID any    `json:"run_id" required:"true" schema:"number"`
	Page  any    `json:"page" schema:"number"`
	Limit any    `json:"limit" schema:"number"`
	Name  string `json:"name"`
}

// getArtifactArgs are the arguments of get_action_artifact.
type getArtifactArgs struct {
	Owner      string `json:"owner" required:"true"`
	Repo       string `json:"repo" required:"true"`
	ArtifactID any    `json:"artifact_id" required:"true" schema:"number"`
}

const (
	ListActionRunArtifactsToolName = "list_action_run_artifacts"
	GetActionArtifactToolName      = "get_action_artifact"

	defaultActionArtifactsLimit = 30
	maxActionArtifactsLimit     = 50
)

var (
	ListActionRunArtifactsTool = mcp.NewTool(
		ListActionRunArtifactsToolName,
		mcp.WithDescription("List artifacts of a workflow run. Server-paged: page (default 1) and limit (default 30, maximum 50) are sent as query parameters, optional name filters by artifact name. Returns {run_id, artifacts, page, limit, count, has_next} and total_count when Forgejo sets X-Total-Count; has_next true means a further page may exist, so re-issue the call with page incremented. A missing run is an error, not an empty list. Metadata only — does not download zips."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("run_id", mcp.Required(), mcp.Description(params.RunID), mcp.Min(1)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(defaultActionArtifactsLimit), mcp.Min(1), mcp.Max(maxActionArtifactsLimit)),
		mcp.WithString("name", mcp.Description(params.ArtifactName)),
	)

	GetActionArtifactTool = mcp.NewTool(
		GetActionArtifactToolName,
		mcp.WithDescription("Get metadata for one Actions artifact (id, name, run_id, size_in_bytes, expired, timestamps, archive_download_url). Does not download the zip."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("artifact_id", mcp.Required(), mcp.Description(params.ArtifactID), mcp.Min(1)),
	)
)

func (h handlers) ListActionRunArtifactsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var a artifactsArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	if err := requiredRepo(a.Owner, a.Repo); err != nil {
		return to.ErrorResult(err)
	}
	runID, err := positiveIntegerArg(a.RunID, "run_id", true)
	if err != nil {
		return to.ErrorResult(err)
	}
	page, err := boundedIntArg(a.Page, "page", 1, 1, math.MaxInt32)
	if err != nil {
		return to.ErrorResult(err)
	}
	limit, err := boundedIntArg(a.Limit, "limit", 0, 1, maxActionArtifactsLimit)
	if err != nil {
		return to.ErrorResult(err)
	}

	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	list, err := svc.ListArtifacts(ctx, a.Owner, a.Repo, runID, app.ArtifactQuery{
		Paging: app.Paging{Page: page, Limit: limit},
		Name:   a.Name,
	})
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(list)
}

func (h handlers) GetActionArtifactFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetActionArtifactFn")

	var a getArtifactArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	if err := requiredRepo(a.Owner, a.Repo); err != nil {
		return to.ErrorResult(err)
	}
	artifactID, err := positiveIntegerArg(a.ArtifactID, "artifact_id", true)
	if err != nil {
		return to.ErrorResult(err)
	}

	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	artifact, err := svc.GetArtifact(ctx, a.Owner, a.Repo, artifactID)
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(artifact)
}
