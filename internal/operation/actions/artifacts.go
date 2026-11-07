// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/numarg"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/decode"
	"math"

	app "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/actions"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/annotate"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcpresult"
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
		annotate.Read(),
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
		annotate.Read(),
		mcp.WithDescription("Get metadata for one Actions artifact (id, name, run_id, size_in_bytes, expired, timestamps, archive_download_url). Does not download the zip."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("artifact_id", mcp.Required(), mcp.Description(params.ArtifactID), mcp.Min(1)),
	)
)

// Validate checks owner and repo, then run_id and the paging arguments, before
// the credential lookup.
func (a artifactsArgs) Validate() error {
	if err := decode.RequiredRepo(a.Owner, a.Repo); err != nil {
		return err
	}
	_, _, err := a.read()
	return err
}

// read reads run_id and the paging arguments.
func (a artifactsArgs) read() (int64, app.Paging, error) {
	runID, err := numarg.RequiredPositiveInt("run_id", a.RunID)
	if err != nil {
		return 0, app.Paging{}, err
	}
	page, err := numarg.IntDefault("page", a.Page, 1, numarg.Bounds{Min: int64(1), Max: int64(math.MaxInt32)})
	if err != nil {
		return 0, app.Paging{}, err
	}
	limit, err := numarg.IntDefault("limit", a.Limit, 0, numarg.Bounds{Min: int64(1), Max: int64(maxActionArtifactsLimit)})
	if err != nil {
		return 0, app.Paging{}, err
	}
	return runID, app.Paging{Page: page, Limit: limit}, nil
}

func listActionRunArtifacts(ctx context.Context, svc *app.Service, a artifactsArgs) (*mcp.CallToolResult, error) {
	runID, paging, err := a.read()
	if err != nil {
		return mcpresult.Fail(err)
	}
	list, err := svc.ListArtifacts(ctx, a.Owner, a.Repo, runID, app.ArtifactQuery{
		Paging: paging,
		Name:   a.Name,
	})
	if err != nil {
		return mcpresult.Fail(err)
	}
	return to.TextResult(artifactPage(runID, list))
}

// Validate checks owner and repo, then artifact_id, before the credential lookup.
func (a getArtifactArgs) Validate() error {
	if err := decode.RequiredRepo(a.Owner, a.Repo); err != nil {
		return err
	}
	_, err := numarg.RequiredPositiveInt("artifact_id", a.ArtifactID)
	return err
}

func getActionArtifact(ctx context.Context, svc *app.Service, a getArtifactArgs) (*mcp.CallToolResult, error) {
	artifactID, err := numarg.RequiredPositiveInt("artifact_id", a.ArtifactID)
	if err != nil {
		return mcpresult.Fail(err)
	}
	artifact, err := svc.GetArtifact(ctx, a.Owner, a.Repo, artifactID)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return to.TextResult(artifact)
}
