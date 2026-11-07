// SPDX-License-Identifier: GPL-3.0-or-later

package tracking

import (
	"context"
	"errors"
	"math"

	apptracking "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/tracking"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/trackingadapter"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/numarg"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/decode"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcpresult"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/to"

	"github.com/mark3labs/mcp-go/mcp"
)

// handlers serve the time-tracking tools. They hold the factory rather than a
// port so each call is bound to the credential of its own request. Each tool is
// the MCP adapter of one use case in internal/app/tracking: the pipeline decodes
// and validates the arguments, the use case runs, and the result is rendered.
type handlers struct{ forge forge.Factory }

// Argument structs, one per tool. `json` names the MCP argument and
// `required:"true"` marks the required ones; mcptest.ArgsMatchSchema keeps
// them in agreement with the tool definitions. An ID or seconds field is kept
// raw (any) and read by the numeric-argument module in Validate, before the
// use case runs, so the use case receives a validated whole number.

type issueArgs struct {
	Owner string `json:"owner" required:"true"`
	Repo  string `json:"repo" required:"true"`
	Index any    `json:"index" required:"true" schema:"number"`
}

type issueTimesArgs struct {
	Owner  string  `json:"owner" required:"true"`
	Repo   string  `json:"repo" required:"true"`
	Index  any     `json:"index" required:"true" schema:"number"`
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
	Owner     string `json:"owner" required:"true"`
	Repo      string `json:"repo" required:"true"`
	Index     any    `json:"index" required:"true" schema:"number"`
	Seconds   any    `json:"seconds" schema:"number"`
	Duration  string `json:"duration"`
	CreatedAt string `json:"created_at"`
	UserName  string `json:"user_name"`
}

type deleteEntryArgs struct {
	Owner  string `json:"owner" required:"true"`
	Repo   string `json:"repo" required:"true"`
	Index  any    `json:"index" required:"true" schema:"number"`
	TimeID any    `json:"time_id" required:"true" schema:"number"`
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

// service is the Binder for every time-tracking tool. The use cases reach the
// credential's port only after their input is checked (see apptracking.Source),
// so binding does not look up a credential.
func (h handlers) service(context.Context) (*apptracking.Service, error) {
	return apptracking.New(h.source()), nil
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

// requireRepo refuses a call without its owner or repository, in the order the
// arguments are read: owner, then repo.
func requireRepo(owner, repo string) error {
	if owner == "" {
		return errors.New("owner is required")
	}
	if repo == "" {
		return errors.New("repo is required")
	}
	return nil
}

// positive is the range of an ID or of a count of seconds: a whole number of
// at least one.
var positive = numarg.Bounds{Min: 1, Max: math.MaxInt64}

// issueRef reads an issue's reference from a call's owner, repo and index,
// refusing an index that is not a positive whole number before any use case
// runs.
func issueRef(owner, repo string, index any) (apptracking.IssueRef, error) {
	i, err := numarg.RequiredInt("index", index, positive)
	if err != nil {
		return apptracking.IssueRef{}, err
	}
	return apptracking.IssueRef{Owner: owner, Repo: repo, Index: i}, nil
}

// timeEntryID reads a time entry's ID, refusing anything but a positive whole
// number.
func timeEntryID(raw any) (int64, error) {
	return numarg.RequiredInt("time_id", raw, positive)
}

// optionalSeconds reads the seconds of a time entry: nil when not given, else
// the whole number of seconds, a fraction truncated. A value that truncates to
// less than one second is refused.
func optionalSeconds(raw any) (*int64, error) {
	v, given, err := numarg.Whole("seconds", raw, positive)
	if err != nil || !given {
		return nil, err
	}
	return &v, nil
}

// Validate checks the repository, then the index, before the credential lookup.
func (a issueArgs) Validate() error {
	if err := requireRepo(a.Owner, a.Repo); err != nil {
		return err
	}
	_, err := issueRef(a.Owner, a.Repo, a.Index)
	return err
}

// Validate checks the repository, the index, then the paging arguments, before
// the credential lookup.
func (a issueTimesArgs) Validate() error {
	if err := requireRepo(a.Owner, a.Repo); err != nil {
		return err
	}
	if _, err := issueRef(a.Owner, a.Repo, a.Index); err != nil {
		return err
	}
	_, _, err := paging(a.Page, a.Limit)
	return err
}

// Validate checks the repository, then the paging arguments, before the
// credential lookup.
func (a repoTimesArgs) Validate() error {
	if err := requireRepo(a.Owner, a.Repo); err != nil {
		return err
	}
	_, _, err := paging(a.Page, a.Limit)
	return err
}

// Validate checks the repository, the index, then the seconds, before the
// credential lookup.
func (a addTimeArgs) Validate() error {
	if err := requireRepo(a.Owner, a.Repo); err != nil {
		return err
	}
	if _, err := issueRef(a.Owner, a.Repo, a.Index); err != nil {
		return err
	}
	_, err := optionalSeconds(a.Seconds)
	return err
}

// Validate checks the repository, the index, then the time entry's ID, before
// the credential lookup.
func (a deleteEntryArgs) Validate() error {
	if err := requireRepo(a.Owner, a.Repo); err != nil {
		return err
	}
	if _, err := issueRef(a.Owner, a.Repo, a.Index); err != nil {
		return err
	}
	_, err := timeEntryID(a.TimeID)
	return err
}

// Validate checks the paging arguments before the credential lookup.
func (a myTimesArgs) Validate() error {
	_, _, err := paging(a.Page, a.Limit)
	return err
}

// Validate checks the paging arguments before the credential lookup.
func (a myStopwatchesArgs) Validate() error {
	_, _, err := paging(a.Page, a.Limit)
	return err
}

func listIssueTrackedTimes(ctx context.Context, s *apptracking.Service, a issueTimesArgs) (*mcp.CallToolResult, error) {
	issue, err := issueRef(a.Owner, a.Repo, a.Index)
	if err != nil {
		return mcpresult.Fail(err)
	}
	page, limit, err := paging(a.Page, a.Limit)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return mcpresult.Render(s.IssueTimes(ctx, apptracking.IssueTimesInput{
		IssueRef: issue,
		Since:    a.Since, Before: a.Before, Page: page, Limit: limit,
	}))
}

func listRepoTrackedTimes(ctx context.Context, s *apptracking.Service, a repoTimesArgs) (*mcp.CallToolResult, error) {
	page, limit, err := paging(a.Page, a.Limit)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return mcpresult.Render(s.RepoTimes(ctx, apptracking.RepoTimesInput{
		Owner: a.Owner, Repo: a.Repo, Since: a.Since, Before: a.Before, User: a.User, Page: page, Limit: limit,
	}))
}

func listMyTrackedTimes(ctx context.Context, s *apptracking.Service, a myTimesArgs) (*mcp.CallToolResult, error) {
	page, limit, err := paging(a.Page, a.Limit)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return mcpresult.Render(s.MyTimes(ctx, apptracking.MyTimesInput{Page: page, Limit: limit}))
}

func addIssueTime(ctx context.Context, s *apptracking.Service, a addTimeArgs) (*mcp.CallToolResult, error) {
	issue, err := issueRef(a.Owner, a.Repo, a.Index)
	if err != nil {
		return mcpresult.Fail(err)
	}
	seconds, err := optionalSeconds(a.Seconds)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return mcpresult.Render(s.AddTime(ctx, apptracking.AddTimeInput{
		IssueRef:  issue,
		Seconds:   seconds,
		Duration:  a.Duration,
		CreatedAt: a.CreatedAt,
		User:      a.UserName,
	}))
}

func resetIssueTime(ctx context.Context, s *apptracking.Service, a issueArgs) (*mcp.CallToolResult, error) {
	issue, err := issueRef(a.Owner, a.Repo, a.Index)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return message("Reset tracked time success", s.ResetTime(ctx, issue))
}

func deleteIssueTimeEntry(ctx context.Context, s *apptracking.Service, a deleteEntryArgs) (*mcp.CallToolResult, error) {
	issue, err := issueRef(a.Owner, a.Repo, a.Index)
	if err != nil {
		return mcpresult.Fail(err)
	}
	id, err := timeEntryID(a.TimeID)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return message("Delete time entry success", s.DeleteTimeEntry(ctx, issue, id))
}

func startIssueStopwatch(ctx context.Context, s *apptracking.Service, a issueArgs) (*mcp.CallToolResult, error) {
	issue, err := issueRef(a.Owner, a.Repo, a.Index)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return message("Stopwatch started", s.StartStopwatch(ctx, issue))
}

func stopIssueStopwatch(ctx context.Context, s *apptracking.Service, a issueArgs) (*mcp.CallToolResult, error) {
	issue, err := issueRef(a.Owner, a.Repo, a.Index)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return message("Stopwatch stopped; elapsed time recorded as a tracked time entry", s.StopStopwatch(ctx, issue))
}

func cancelIssueStopwatch(ctx context.Context, s *apptracking.Service, a issueArgs) (*mcp.CallToolResult, error) {
	issue, err := issueRef(a.Owner, a.Repo, a.Index)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return message("Stopwatch cancelled", s.CancelStopwatch(ctx, issue))
}

func listMyStopwatches(ctx context.Context, s *apptracking.Service, a myStopwatchesArgs) (*mcp.CallToolResult, error) {
	page, limit, err := paging(a.Page, a.Limit)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return mcpresult.Render(s.MyStopwatches(ctx, apptracking.MyStopwatchesInput{Page: page, Limit: limit}))
}
