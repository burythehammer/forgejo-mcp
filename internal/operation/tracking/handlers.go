// SPDX-License-Identifier: GPL-3.0-or-later

package tracking

import (
	"context"

	apptracking "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/tracking"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/trackingadapter"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/log"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/decode"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcpresult"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/to"

	"github.com/mark3labs/mcp-go/mcp"
)

// handlers serve the time-tracking tools. They hold the factory rather than a
// port so each call is bound to the credential of its own request. Each handler
// is the MCP adapter of one use case in internal/app/tracking: decode the arguments, run
// the use case, render the result.
type handlers struct{ forge forge.Factory }

// Argument structs, one per tool. `json` names the MCP argument and
// `required:"true"` marks the required ones; mcptest.ArgsMatchSchema keeps
// them in agreement with the tool definitions. Numbers are float64 so the
// forgiving decoder truncates them as the original handlers did. seconds is a
// pointer so that a missing, null or unparseable value reads as absent, as the
// original's Float64Ok did.

type issueArgs struct {
	Owner string  `json:"owner" required:"true"`
	Repo  string  `json:"repo" required:"true"`
	Index float64 `json:"index" required:"true"`
}

type issueTimesArgs struct {
	Owner  string  `json:"owner" required:"true"`
	Repo   string  `json:"repo" required:"true"`
	Index  float64 `json:"index" required:"true"`
	Since  string  `json:"since"`
	Before string  `json:"before"`
	Page   float64 `json:"page"`
	Limit  float64 `json:"limit"`
}

type repoTimesArgs struct {
	Owner  string  `json:"owner" required:"true"`
	Repo   string  `json:"repo" required:"true"`
	Since  string  `json:"since"`
	Before string  `json:"before"`
	User   string  `json:"user"`
	Page   float64 `json:"page"`
	Limit  float64 `json:"limit"`
}

type addTimeArgs struct {
	Owner     string   `json:"owner" required:"true"`
	Repo      string   `json:"repo" required:"true"`
	Index     float64  `json:"index" required:"true"`
	Seconds   *float64 `json:"seconds" numeric:"string"`
	Duration  string   `json:"duration"`
	CreatedAt string   `json:"created_at"`
	UserName  string   `json:"user_name"`
}

type deleteEntryArgs struct {
	Owner  string  `json:"owner" required:"true"`
	Repo   string  `json:"repo" required:"true"`
	Index  float64 `json:"index" required:"true"`
	TimeID float64 `json:"time_id" required:"true"`
}

// myTimesArgs are list_my_tracked_times' arguments: the page to read.
type myTimesArgs struct {
	Page  float64 `json:"page"`
	Limit float64 `json:"limit"`
}

// myStopwatchesArgs are list_my_stopwatches' arguments: the page to read.
type myStopwatchesArgs struct {
	Page  float64 `json:"page"`
	Limit float64 `json:"limit"`
}

// paging reads the page and limit arguments through the numeric-argument
// module (decode.PagingArgs): a zero page is page 1, an absent limit is 0 for
// the use case's default, and a value that is not a number is refused.
func paging(page, limit float64) (int, int, error) {
	return decode.PagingArgs(page, limit)
}

// run is the shared shape of a tool call: decode the arguments into A, then
// hand them and the use cases to do. The use cases reach the credential's port
// only after their input is checked (see apptracking.Source).
func run[A any](ctx context.Context, h handlers, req mcp.CallToolRequest, name string,
	do func(context.Context, *apptracking.Service, A) (*mcp.CallToolResult, error),
) (*mcp.CallToolResult, error) {
	log.Debugf("Called %s", name)
	var args A
	if err := decode.Lax(req, &args); err != nil {
		return mcpresult.Fail(err)
	}
	return do(ctx, apptracking.New(h.source()), args)
}

// source returns the lookup of this request's port, which the use cases call
// once their input is checked.
func (h handlers) source() apptracking.Source {
	return func(ctx context.Context) (apptracking.Port, error) {
		port, err := h.forge.For(ctx)
		if err != nil {
			return nil, err
		}
		return trackingadapter.New(port), nil
	}
}

// message sends a use case's outcome as the given text.
func message(text string, err error) (*mcp.CallToolResult, error) {
	if err != nil {
		return mcpresult.Fail(err)
	}
	return to.TextResult(text)
}

func ref(a issueArgs) apptracking.IssueRef {
	return apptracking.IssueRef{Owner: a.Owner, Repo: a.Repo, Index: int64(a.Index)}
}

func (h handlers) ListIssueTrackedTimesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "ListIssueTrackedTimesFn", func(ctx context.Context, s *apptracking.Service, a issueTimesArgs) (*mcp.CallToolResult, error) {
		page, limit, err := paging(a.Page, a.Limit)
		if err != nil {
			return mcpresult.Fail(err)
		}
		return mcpresult.Render(s.IssueTimes(ctx, apptracking.IssueTimesInput{
			IssueRef: ref(issueArgs{Owner: a.Owner, Repo: a.Repo, Index: a.Index}),
			Since:    a.Since, Before: a.Before, Page: page, Limit: limit,
		}))
	})
}

func (h handlers) ListRepoTrackedTimesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "ListRepoTrackedTimesFn", func(ctx context.Context, s *apptracking.Service, a repoTimesArgs) (*mcp.CallToolResult, error) {
		page, limit, err := paging(a.Page, a.Limit)
		if err != nil {
			return mcpresult.Fail(err)
		}
		return mcpresult.Render(s.RepoTimes(ctx, apptracking.RepoTimesInput{
			Owner: a.Owner, Repo: a.Repo, Since: a.Since, Before: a.Before, User: a.User, Page: page, Limit: limit,
		}))
	})
}

func (h handlers) ListMyTrackedTimesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "ListMyTrackedTimesFn", func(ctx context.Context, s *apptracking.Service, a myTimesArgs) (*mcp.CallToolResult, error) {
		page, limit, err := paging(a.Page, a.Limit)
		if err != nil {
			return mcpresult.Fail(err)
		}
		return mcpresult.Render(s.MyTimes(ctx, apptracking.MyTimesInput{Page: page, Limit: limit}))
	})
}

func (h handlers) AddIssueTimeFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "AddIssueTimeFn", func(ctx context.Context, s *apptracking.Service, a addTimeArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render(s.AddTime(ctx, apptracking.AddTimeInput{
			IssueRef:  apptracking.IssueRef{Owner: a.Owner, Repo: a.Repo, Index: int64(a.Index)},
			Seconds:   a.Seconds,
			Duration:  a.Duration,
			CreatedAt: a.CreatedAt,
			User:      a.UserName,
		}))
	})
}

func (h handlers) ResetIssueTimeFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "ResetIssueTimeFn", func(ctx context.Context, s *apptracking.Service, a issueArgs) (*mcp.CallToolResult, error) {
		return message("Reset tracked time success", s.ResetTime(ctx, ref(a)))
	})
}

func (h handlers) DeleteIssueTimeEntryFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "DeleteIssueTimeEntryFn", func(ctx context.Context, s *apptracking.Service, a deleteEntryArgs) (*mcp.CallToolResult, error) {
		return message("Delete time entry success", s.DeleteTimeEntry(ctx, ref(issueArgs{Owner: a.Owner, Repo: a.Repo, Index: a.Index}), int64(a.TimeID)))
	})
}

func (h handlers) StartIssueStopwatchFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "StartIssueStopwatchFn", func(ctx context.Context, s *apptracking.Service, a issueArgs) (*mcp.CallToolResult, error) {
		return message("Stopwatch started", s.StartStopwatch(ctx, ref(a)))
	})
}

func (h handlers) StopIssueStopwatchFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "StopIssueStopwatchFn", func(ctx context.Context, s *apptracking.Service, a issueArgs) (*mcp.CallToolResult, error) {
		return message("Stopwatch stopped; elapsed time recorded as a tracked time entry", s.StopStopwatch(ctx, ref(a)))
	})
}

func (h handlers) CancelIssueStopwatchFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "CancelIssueStopwatchFn", func(ctx context.Context, s *apptracking.Service, a issueArgs) (*mcp.CallToolResult, error) {
		return message("Stopwatch cancelled", s.CancelStopwatch(ctx, ref(a)))
	})
}

func (h handlers) ListMyStopwatchesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "ListMyStopwatchesFn", func(ctx context.Context, s *apptracking.Service, a myStopwatchesArgs) (*mcp.CallToolResult, error) {
		page, limit, err := paging(a.Page, a.Limit)
		if err != nil {
			return mcpresult.Fail(err)
		}
		return mcpresult.Render(s.MyStopwatches(ctx, apptracking.MyStopwatchesInput{Page: page, Limit: limit}))
	})
}
