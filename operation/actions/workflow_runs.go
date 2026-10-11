// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"errors"
	"fmt"
	"strings"

	app "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/actions"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/params"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/to"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	ListWorkflowRunsToolName = "list_workflow_runs"
	GetWorkflowRunToolName   = "get_workflow_run"
)

var (
	ListWorkflowRunsTool = mcp.NewTool(
		ListWorkflowRunsToolName,
		mcp.WithDescription("List workflow runs for a repository"),
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

	owner, ok := req.GetArguments()["owner"].(string)
	if !ok || owner == "" {
		return to.ErrorResult(errors.New("owner is required"))
	}
	repo, ok := req.GetArguments()["repo"].(string)
	if !ok || repo == "" {
		return to.ErrorResult(errors.New("repo is required"))
	}
	status, _ := req.GetArguments()["status"].(string)
	event, _ := req.GetArguments()["event"].(string)
	headSHA, _ := req.GetArguments()["head_sha"].(string)
	var runNumber int64
	if rn, err := to.Float64(req.GetArguments()["run_number"]); err == nil {
		runNumber = int64(rn)
	}
	page := 1
	if p, err := to.Float64(req.GetArguments()["page"]); err == nil {
		page = int(p)
	}
	limit := 30
	if l, err := to.Float64(req.GetArguments()["limit"]); err == nil {
		limit = int(l)
	}

	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	list, err := svc.ListRuns(ctx, owner, repo, app.RunQuery{
		Status: status, Event: event, RunNumber: runNumber, HeadSHA: headSHA,
		Paging: app.Paging{Page: page, Limit: limit},
	})
	if err != nil {
		return to.ErrorResult(err)
	}
	if len(list.WorkflowRuns) == 0 {
		return to.TextResult(fmt.Sprintf("No workflow runs found for %s/%s", owner, repo))
	}
	return to.TextResult(renderRuns(owner, repo, list))
}

// renderRuns is the list_workflow_runs text.
func renderRuns(owner, repo string, list model.WorkflowRunList) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Workflow Runs for %s/%s (total: %d):\n\n", owner, repo, list.TotalCount)
	for _, run := range list.WorkflowRuns {
		duration := ""
		if !run.Started.IsZero() && !run.Stopped.IsZero() {
			duration = fmt.Sprintf(" | Duration: %s", run.Stopped.Sub(run.Started).Round(1e9).String())
		}
		fmt.Fprintf(&sb, "#%d - %s\n", run.ID, run.Title)
		fmt.Fprintf(&sb, "  Status: %s | Event: %s | SHA: %.7s%s\n",
			run.Status, run.Event, run.CommitSHA, duration)
		if !run.Started.IsZero() {
			fmt.Fprintf(&sb, "  Started: %s\n", run.Started.Format("2006-01-02 15:04:05"))
		}
		fmt.Fprintf(&sb, "  URL: %s\n\n", run.HTMLURL)
	}
	return sb.String()
}

func (h handlers) GetWorkflowRunFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetWorkflowRunFn")

	owner, ok := req.GetArguments()["owner"].(string)
	if !ok || owner == "" {
		return to.ErrorResult(errors.New("owner is required"))
	}
	repo, ok := req.GetArguments()["repo"].(string)
	if !ok || repo == "" {
		return to.ErrorResult(errors.New("repo is required"))
	}
	runIDFloat, err := to.Float64(req.GetArguments()["run_id"])
	if err != nil {
		return to.ErrorResult(errors.New("run_id is required"))
	}
	runID := int64(runIDFloat)
	if runID <= 0 {
		return to.ErrorResult(errors.New("run_id must be a positive integer"))
	}

	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	run, err := svc.GetRun(ctx, owner, repo, runID)
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
