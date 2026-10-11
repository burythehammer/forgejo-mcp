// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"math"

	app "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/actions"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/params"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/to"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	ListActionRunArtifactsToolName = "list_action_run_artifacts"
	GetActionArtifactToolName      = "get_action_artifact"

	defaultActionArtifactsLimit = 30
	maxActionArtifactsLimit     = 50
)

var (
	ListActionRunArtifactsTool = mcp.NewTool(
		ListActionRunArtifactsToolName,
		mcp.WithDescription("List artifacts of a workflow run. Server-paged: page (default 1) and limit (default 30, maximum 50) are sent as query parameters, optional name filters by artifact name. Returns {run_id, artifacts, page, limit, count} and total_count when Forgejo sets X-Total-Count. A missing run is an error, not an empty list. Metadata only — does not download zips."),
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
	args := req.GetArguments()
	owner, repo, err := requiredRepo(args)
	if err != nil {
		return to.ErrorResult(err)
	}
	runID, err := positiveIntegerArg(args, "run_id", true)
	if err != nil {
		return to.ErrorResult(err)
	}
	page, err := to.BoundedInt(args, "page", 1, 1, math.MaxInt32)
	if err != nil {
		return to.ErrorResult(err)
	}
	limit, err := to.BoundedInt(args, "limit", defaultActionArtifactsLimit, 1, maxActionArtifactsLimit)
	if err != nil {
		return to.ErrorResult(err)
	}
	name, _ := args["name"].(string)

	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	list, err := svc.ListArtifacts(ctx, owner, repo, runID, app.ArtifactQuery{
		Paging: app.Paging{Page: page, Limit: limit},
		Name:   name,
	})
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(list)
}

func (h handlers) GetActionArtifactFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetActionArtifactFn")

	args := req.GetArguments()
	owner, repo, err := requiredRepo(args)
	if err != nil {
		return to.ErrorResult(err)
	}
	artifactID, err := positiveIntegerArg(args, "artifact_id", true)
	if err != nil {
		return to.ErrorResult(err)
	}

	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	artifact, err := svc.GetArtifact(ctx, owner, repo, artifactID)
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(artifact)
}
