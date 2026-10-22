// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"fmt"

	app "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/actions"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/log"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/decode"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/params"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/to"

	"github.com/mark3labs/mcp-go/mcp"
)

// listRunsArgs are the arguments of list_workflow_runs.
type listRunsArgs struct {
	Owner     string `json:"owner" required:"true"`
	Repo      string `json:"repo" required:"true"`
	Status    string `json:"status"`
	Event     string `json:"event"`
	RunNumber any    `json:"run_number" schema:"number"`
	HeadSHA   string `json:"head_sha"`
	Page      any    `json:"page" schema:"number"`
	Limit     any    `json:"limit" schema:"number"`
}

// repoRunArgs are the arguments of the tools that act on one run.
type repoRunArgs struct {
	Owner string `json:"owner" required:"true"`
	Repo  string `json:"repo" required:"true"`
	RunID any    `json:"run_id" required:"true" schema:"number"`
}

const (
	ListWorkflowRunsToolName = "list_workflow_runs"
	GetWorkflowRunToolName   = "get_workflow_run"
)

var (
	ListWorkflowRunsTool = mcp.NewTool(
		ListWorkflowRunsToolName,
		mcp.WithDescription("List workflow runs for a repository. Returns a response envelope {workflow_runs, page, limit, count, total_count, has_next}; total_count is present only when Forgejo reports it. has_next true means a further page may exist; re-issue the call with page incremented to fetch it."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("status", mcp.Description(params.Status)),
		mcp.WithString("event", mcp.Description(params.Event)),
		mcp.WithNumber("run_number", mcp.Description(params.RunNumber)),
		mcp.WithString("head_sha", mcp.Description(params.HeadSHA)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(30), mcp.Min(1)),
	)

	GetWorkflowRunTool = mcp.NewTool(
		GetWorkflowRunToolName,
		mcp.WithDescription("Get details of a specific workflow run"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("run_id", mcp.Required(), mcp.Description(params.RunID)),
	)
)

func (h handlers) ListWorkflowRunsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListWorkflowRunsFn")

	var a listRunsArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	if err := requiredRepo(a.Owner, a.Repo); err != nil {
		return to.ErrorResult(err)
	}
	runNumber, _, err := optionalNonNegativeIntegerArg(a.RunNumber, "run_number")
	if err != nil {
		return to.ErrorResult(err)
	}
	// page and limit are still read here, silently defaulted, until the
	// page/limit migration (#244) moves them to the module.
	page := 1
	if p, ok := number(a.Page); ok {
		page = int(p)
	}
	limit := 0
	if l, ok := number(a.Limit); ok {
		limit = int(l)
	}

	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	out, err := svc.ListRuns(ctx, a.Owner, a.Repo, app.RunQuery{
		Status: a.Status, Event: a.Event, RunNumber: runNumber, HeadSHA: a.HeadSHA,
		Paging: app.Paging{Page: page, Limit: limit},
	})
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.SafeTextResult(runPage(out))
}

func (h handlers) GetWorkflowRunFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetWorkflowRunFn")

	var a repoRunArgs
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

	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	run, err := svc.GetRun(ctx, a.Owner, a.Repo, runID)
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(renderRun(run))
}

// renderRun is the get_workflow_run text.
func renderRun(run model.WorkflowRun) string {
	duration := "N/A"
	if !run.Started.IsZero() && !run.Stopped.IsZero() {
		duration = run.Stopped.Sub(run.Started).Round(1e9).String()
	}
	triggerUser := "unknown"
	if run.TriggerUser != nil {
		triggerUser = *run.TriggerUser
	}
	createdStr := "N/A"
	if !run.Created.IsZero() {
		createdStr = run.Created.Format("2006-01-02 15:04:05")
	}
	startedStr := "Not started"
	if !run.Started.IsZero() {
		startedStr = run.Started.Format("2006-01-02 15:04:05")
	}
	stoppedStr := "Not stopped"
	if !run.Stopped.IsZero() {
		stoppedStr = run.Stopped.Format("2006-01-02 15:04:05")
	}
	return fmt.Sprintf(`Workflow Run #%d

Title: %s
Status: %s
Event: %s
Ref: %s
Commit SHA: %s
Triggered by: %s

Created: %s
Started: %s
Stopped: %s
Duration: %s

URL: %s`,
		run.ID,
		run.Title,
		run.Status,
		run.Event,
		run.PrettyRef,
		run.CommitSHA,
		triggerUser,
		createdStr,
		startedStr,
		stoppedStr,
		duration,
		run.HTMLURL,
	)
}
