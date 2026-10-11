// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/params"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/to"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	CancelWorkflowRunToolName = "cancel_workflow_run"
	DeleteWorkflowRunToolName = "delete_workflow_run"
)

type workflowRunControlResult struct {
	RunID  int64  `json:"run_id"`
	Status string `json:"status"`
}

var (
	CancelWorkflowRunTool = mcp.NewTool(
		CancelWorkflowRunToolName,
		mcp.WithDescription("Cancel a pending or running workflow run. Forgejo also returns 204 for a run that has already finished and leaves that run unchanged; that is success, not an error. Same Actions write as dispatch_workflow; a read-only token returns 403."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("run_id", mcp.Required(), mcp.Description(params.RunID), mcp.Min(1)),
	)

	DeleteWorkflowRunTool = mcp.NewTool(
		DeleteWorkflowRunToolName,
		mcp.WithDescription("Delete a completed workflow run (succeeded, failed, or cancelled). The run is removed, its job logs become unreadable, and Forgejo marks that run's artifacts deleted (storage reclaimed later). A live run is an API error (never mapped to success). No preflight GET. Same Actions write as dispatch_workflow; a read-only token returns 403."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("run_id", mcp.Required(), mcp.Description(params.RunID), mcp.Min(1)),
	)
)

func (h handlers) CancelWorkflowRunFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CancelWorkflowRunFn")

	args := req.GetArguments()
	owner, repo, err := requiredRepo(args)
	if err != nil {
		return to.ErrorResult(err)
	}
	runID, err := positiveIntegerArg(args, "run_id", true)
	if err != nil {
		return to.ErrorResult(err)
	}

	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	if err := svc.CancelRun(ctx, owner, repo, runID); err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(workflowRunControlResult{RunID: runID, Status: "cancelled"})
}

func (h handlers) DeleteWorkflowRunFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteWorkflowRunFn")

	args := req.GetArguments()
	owner, repo, err := requiredRepo(args)
	if err != nil {
		return to.ErrorResult(err)
	}
	runID, err := positiveIntegerArg(args, "run_id", true)
	if err != nil {
		return to.ErrorResult(err)
	}

	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	if err := svc.DeleteRun(ctx, owner, repo, runID); err != nil {
		return to.ErrorResult(err)
	}
	return to.TextResult(workflowRunControlResult{RunID: runID, Status: "deleted"})
}
