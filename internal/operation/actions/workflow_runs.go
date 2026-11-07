// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"fmt"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/numarg"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/decode"

	app "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/actions"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/annotate"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcpresult"
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
		annotate.Read(),
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
		annotate.Read(),
		mcp.WithDescription("Get details of a specific workflow run"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("run_id", mcp.Required(), mcp.Description(params.RunID)),
	)
)

// Validate checks owner and repo, then the numeric arguments, before the
// credential lookup.
func (a listRunsArgs) Validate() error {
	if err := decode.RequiredRepo(a.Owner, a.Repo); err != nil {
		return err
	}
	_, err := a.query()
	return err
}

// query reads the filter and paging arguments. A page of 0 is page 1.
func (a listRunsArgs) query() (app.RunQuery, error) {
	runNumber, _, err := numarg.NonNegativeInt("run_number", a.RunNumber)
	if err != nil {
		return app.RunQuery{}, err
	}
	page, err := numarg.Paging("page", a.Page)
	if err != nil {
		return app.RunQuery{}, err
	}
	if page == 0 {
		page = 1
	}
	limit, err := numarg.Paging("limit", a.Limit)
	if err != nil {
		return app.RunQuery{}, err
	}
	return app.RunQuery{
		Status: a.Status, Event: a.Event, RunNumber: runNumber, HeadSHA: a.HeadSHA,
		Paging: app.Paging{Page: page, Limit: limit},
	}, nil
}

func listWorkflowRuns(ctx context.Context, svc *app.Service, a listRunsArgs) (*mcp.CallToolResult, error) {
	q, err := a.query()
	if err != nil {
		return mcpresult.Fail(err)
	}
	out, err := svc.ListRuns(ctx, a.Owner, a.Repo, q)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return to.SafeTextResult(runPage(out))
}

// Validate checks owner and repo, then run_id, before the credential lookup.
func (a repoRunArgs) Validate() error {
	if err := decode.RequiredRepo(a.Owner, a.Repo); err != nil {
		return err
	}
	_, err := a.runID()
	return err
}

// runID reads the required run_id as a positive whole number.
func (a repoRunArgs) runID() (int64, error) {
	return numarg.RequiredPositiveInt("run_id", a.RunID)
}

func getWorkflowRun(ctx context.Context, svc *app.Service, a repoRunArgs) (*mcp.CallToolResult, error) {
	runID, err := a.runID()
	if err != nil {
		return mcpresult.Fail(err)
	}
	run, err := svc.GetRun(ctx, a.Owner, a.Repo, runID)
	if err != nil {
		return mcpresult.Fail(err)
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
