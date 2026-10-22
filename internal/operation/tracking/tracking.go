// SPDX-License-Identifier: GPL-3.0-or-later

package tracking

import (
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/params"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	ListIssueTrackedTimesToolName = "list_issue_tracked_times"
	ListRepoTrackedTimesToolName  = "list_repo_tracked_times"
	ListMyTrackedTimesToolName    = "list_my_tracked_times"
	AddIssueTimeToolName          = "add_issue_time"
	ResetIssueTimeToolName        = "reset_issue_time"
	DeleteIssueTimeEntryToolName  = "delete_issue_time_entry"
	StartIssueStopwatchToolName   = "start_issue_stopwatch"
	StopIssueStopwatchToolName    = "stop_issue_stopwatch"
	CancelIssueStopwatchToolName  = "cancel_issue_stopwatch"
	ListMyStopwatchesToolName     = "list_my_stopwatches"

	indexDescIssueOrPR = "Issue or pull request index (Forgejo shares index namespace between the two)"
)

var (
	ListIssueTrackedTimesTool = mcp.NewTool(
		ListIssueTrackedTimesToolName,
		mcp.WithDescription("List tracked time entries on an issue or pull request. Returns a response envelope {times, page, limit, count, has_next}; has_next true means a further page may exist; re-issue the call with page incremented to fetch it."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(indexDescIssueOrPR)),
		mcp.WithString("since", mcp.Description(params.Since)),
		mcp.WithString("before", mcp.Description(params.Before)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(20)),
	)

	ListRepoTrackedTimesTool = mcp.NewTool(
		ListRepoTrackedTimesToolName,
		mcp.WithDescription("List tracked time entries across all issues and PRs in a repository. Returns a response envelope {times, page, limit, count, has_next}; has_next true means a further page may exist; re-issue the call with page incremented to fetch it."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("since", mcp.Description(params.Since)),
		mcp.WithString("before", mcp.Description(params.Before)),
		mcp.WithString("user", mcp.Description(params.TimeUserFilter)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(20)),
	)

	ListMyTrackedTimesTool = mcp.NewTool(
		ListMyTrackedTimesToolName,
		mcp.WithDescription("List tracked time entries for the authenticated user across all repositories"),
	)

	AddIssueTimeTool = mcp.NewTool(
		AddIssueTimeToolName,
		mcp.WithDescription("Log time against an issue or pull request, at least one whole second. Provide exactly one of 'seconds' or 'duration'."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(indexDescIssueOrPR)),
		mcp.WithNumber("seconds", mcp.Description(params.TimeSeconds)),
		mcp.WithString("duration", mcp.Description(params.TimeDuration)),
		mcp.WithString("created_at", mcp.Description(params.TimeCreatedAt)),
		mcp.WithString("user_name", mcp.Description(params.TimeUserName)),
	)

	ResetIssueTimeTool = mcp.NewTool(
		ResetIssueTimeToolName,
		mcp.WithDescription("Delete ALL tracked time entries on an issue or pull request (including entries from other users). This is destructive and cannot be undone."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(indexDescIssueOrPR)),
	)

	DeleteIssueTimeEntryTool = mcp.NewTool(
		DeleteIssueTimeEntryToolName,
		mcp.WithDescription("Delete a single tracked time entry by its ID"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(indexDescIssueOrPR)),
		mcp.WithNumber("time_id", mcp.Required(), mcp.Description(params.TimeID)),
	)

	StartIssueStopwatchTool = mcp.NewTool(
		StartIssueStopwatchToolName,
		mcp.WithDescription("Start a stopwatch on an issue or pull request. Only one stopwatch per issue; fails if one is already running."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(indexDescIssueOrPR)),
	)

	StopIssueStopwatchTool = mcp.NewTool(
		StopIssueStopwatchToolName,
		mcp.WithDescription("Stop a running stopwatch and record the elapsed time as a tracked time entry"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(indexDescIssueOrPR)),
	)

	CancelIssueStopwatchTool = mcp.NewTool(
		CancelIssueStopwatchToolName,
		mcp.WithDescription("Cancel a running stopwatch without recording a tracked time entry"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("index", mcp.Required(), mcp.Description(indexDescIssueOrPR)),
	)

	ListMyStopwatchesTool = mcp.NewTool(
		ListMyStopwatchesToolName,
		mcp.WithDescription("List all currently running stopwatches for the authenticated user"),
	)
)

// RegisterTool registers the time-tracking tools. Each call takes its Forgejo
// port from f, so a request is served with the credential that request carries.
func RegisterTool(s *server.MCPServer, f forge.Factory) {
	h := handlers{forge: f}
	s.AddTool(ListIssueTrackedTimesTool, h.ListIssueTrackedTimesFn)
	s.AddTool(ListRepoTrackedTimesTool, h.ListRepoTrackedTimesFn)
	s.AddTool(ListMyTrackedTimesTool, h.ListMyTrackedTimesFn)
	s.AddTool(AddIssueTimeTool, h.AddIssueTimeFn)
	s.AddTool(ResetIssueTimeTool, h.ResetIssueTimeFn)
	s.AddTool(DeleteIssueTimeEntryTool, h.DeleteIssueTimeEntryFn)
	s.AddTool(StartIssueStopwatchTool, h.StartIssueStopwatchFn)
	s.AddTool(StopIssueStopwatchTool, h.StopIssueStopwatchFn)
	s.AddTool(CancelIssueStopwatchTool, h.CancelIssueStopwatchFn)
	s.AddTool(ListMyStopwatchesTool, h.ListMyStopwatchesFn)
}
