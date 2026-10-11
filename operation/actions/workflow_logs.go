// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"math"

	app "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/actions"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/decode"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/params"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/to"

	"github.com/mark3labs/mcp-go/mcp"
)

// jobsArgs are the arguments of list_action_run_jobs.
type jobsArgs struct {
	Owner string `json:"owner" required:"true"`
	Repo  string `json:"repo" required:"true"`
	RunID any    `json:"run_id" required:"true" schema:"number"`
	Page  any    `json:"page" schema:"number"`
	Limit any    `json:"limit" schema:"number"`
}

// jobLogArgs are the arguments of get_action_job_logs.
type jobLogArgs struct {
	Owner    string `json:"owner" required:"true"`
	Repo     string `json:"repo" required:"true"`
	JobID    any    `json:"job_id" required:"true" schema:"number"`
	Attempt  any    `json:"attempt" schema:"number"`
	Offset   any    `json:"offset" schema:"number"`
	MaxBytes any    `json:"max_bytes" schema:"number"`
}

const (
	ListActionRunJobsToolName = "list_action_run_jobs"
	GetActionJobLogsToolName  = "get_action_job_logs"

	defaultActionJobsLimit = 30
	maxActionJobsLimit     = 100
	defaultActionLogBytes  = 32 * 1024
	maxActionLogBytes      = 256 * 1024
)

var (
	ListActionRunJobsTool = mcp.NewTool(
		ListActionRunJobsToolName,
		mcp.WithDescription("List jobs for a Forgejo v16+ workflow run. Results are paged client-side because Forgejo returns the full job list."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("run_id", mcp.Required(), mcp.Description(params.RunID), mcp.Min(1)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(defaultActionJobsLimit), mcp.Min(1), mcp.Max(maxActionJobsLimit)),
	)

	GetActionJobLogsTool = mcp.NewTool(
		GetActionJobLogsToolName,
		mcp.WithDescription("Read a bounded byte range from a Forgejo v16+ workflow job's plaintext log. Omitting offset returns the tail. Continue backward with previous_offset and previous_max_bytes, or forward with next_offset."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("job_id", mcp.Required(), mcp.Description(params.JobID), mcp.Min(1)),
		mcp.WithNumber("attempt", mcp.Description(params.Attempt), mcp.Min(1)),
		mcp.WithNumber("offset", mcp.Description(params.LogOffset), mcp.Min(0)),
		mcp.WithNumber("max_bytes", mcp.Description(params.LogMaxBytes), mcp.DefaultNumber(defaultActionLogBytes), mcp.Min(1), mcp.Max(maxActionLogBytes)),
	)
)

func (h handlers) ListActionRunJobsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListActionRunJobsFn")

	var a jobsArgs
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
	limit, err := boundedIntArg(a.Limit, "limit", defaultActionJobsLimit, 1, maxActionJobsLimit)
	if err != nil {
		return to.ErrorResult(err)
	}

	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	jobs, err := svc.ListJobs(ctx, a.Owner, a.Repo, runID, app.Paging{Page: page, Limit: limit})
	if err != nil {
		return to.ErrorResult(err)
	}
	return to.SafeTextResult(jobs)
}

func (h handlers) GetActionJobLogsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetActionJobLogsFn")

	var a jobLogArgs
	if err := decode.Lax(req, &a); err != nil {
		return to.ErrorResult(err)
	}
	if err := requiredRepo(a.Owner, a.Repo); err != nil {
		return to.ErrorResult(err)
	}
	jobID, err := positiveIntegerArg(a.JobID, "job_id", true)
	if err != nil {
		return to.ErrorResult(err)
	}
	attempt, err := positiveIntegerArg(a.Attempt, "attempt", false)
	if err != nil {
		return to.ErrorResult(err)
	}
	maxBytes, err := boundedIntArg(a.MaxBytes, "max_bytes", defaultActionLogBytes, 1, maxActionLogBytes)
	if err != nil {
		return to.ErrorResult(err)
	}
	offset, hasOffset, err := optionalNonNegativeIntegerArg(a.Offset, "offset")
	if err != nil {
		return to.ErrorResult(err)
	}

	svc, err := useCases(ctx, h)
	if err != nil {
		return to.ErrorResult(err)
	}
	page, err := svc.JobLog(ctx, a.Owner, a.Repo, app.JobLogQuery{
		JobID: jobID, Attempt: attempt, Offset: offset, HasOffset: hasOffset, MaxBytes: int64(maxBytes),
	})
	if err != nil {
		return to.ErrorResult(err)
	}
	// Do not use TextResult here: it debug-logs the complete payload, and CI
	// logs may contain sensitive output even when Forgejo masks known secrets.
	return to.SafeTextResult(page)
}
