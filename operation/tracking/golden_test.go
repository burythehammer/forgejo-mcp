// SPDX-License-Identifier: GPL-3.0-or-later

package tracking

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/operation/mcptest"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge/fake"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// These tests pin the wire contract of every tracking tool: its tool definition
// (schema snapshot) and its output and error text (golden), together with the
// calls the tool made on the Forgejo port. They run against a stub port that
// returns fixed, fully populated data, so the bytes are deterministic.

var allTools = []mcp.Tool{
	ListIssueTrackedTimesTool, ListRepoTrackedTimesTool, ListMyTrackedTimesTool, AddIssueTimeTool,
	ResetIssueTimeTool, DeleteIssueTimeEntryTool, StartIssueStopwatchTool, StopIssueStopwatchTool,
	CancelIssueStopwatchTool, ListMyStopwatchesTool,
}

func TestSchemaSnapshots(t *testing.T) {
	t.Parallel()
	for _, tool := range allTools {
		t.Run(tool.Name, func(t *testing.T) { mcptest.SchemaSnapshot(t, tool) })
	}
}

var (
	t0   = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	t1   = time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	user = &forgejo_sdk.User{ID: 1, UserName: "octo", FullName: "Octo Cat", Created: t0, LastLogin: t1, IsActive: true}
)

func fixedTime(id int64) *forgejo_sdk.TrackedTime {
	return &forgejo_sdk.TrackedTime{
		ID: id, Created: t0, Time: 5400, UserID: 1, UserName: "octo",
		Issue: &forgejo_sdk.Issue{ID: 11, Index: 7, Title: "fixed", Poster: user, State: forgejo_sdk.StateOpen, Created: t0, Updated: t1},
	}
}

// stubTracking is a forge.Forgejo whose tracking domain returns fixed data and
// records the calls it receives. Any other domain would panic on the nil embed.
type stubTracking struct {
	forge.Forgejo
	mu    sync.Mutex
	calls []string
	fail  error
}

func (s *stubTracking) rec(name string, args ...any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, _ := json.Marshal(args)
	s.calls = append(s.calls, name+" "+string(b))
	return s.fail
}

func (s *stubTracking) ListIssueTrackedTimes(_ context.Context, o, r string, i int64, opt forgejo_sdk.ListTrackedTimesOptions) ([]*forgejo_sdk.TrackedTime, error) {
	return []*forgejo_sdk.TrackedTime{fixedTime(1), fixedTime(2)}, s.rec("ListIssueTrackedTimes", o, r, i, opt)
}

func (s *stubTracking) ListRepoTrackedTimes(_ context.Context, o, r string, opt forgejo_sdk.ListTrackedTimesOptions) ([]*forgejo_sdk.TrackedTime, error) {
	return []*forgejo_sdk.TrackedTime{fixedTime(3)}, s.rec("ListRepoTrackedTimes", o, r, opt)
}

func (s *stubTracking) ListMyTrackedTimes(context.Context) ([]*forgejo_sdk.TrackedTime, error) {
	return []*forgejo_sdk.TrackedTime{fixedTime(4)}, s.rec("ListMyTrackedTimes")
}

func (s *stubTracking) AddIssueTime(_ context.Context, o, r string, i int64, opt forgejo_sdk.AddTimeOption) (*forgejo_sdk.TrackedTime, error) {
	return fixedTime(5), s.rec("AddIssueTime", o, r, i, opt)
}

func (s *stubTracking) ResetIssueTime(_ context.Context, o, r string, i int64) error {
	return s.rec("ResetIssueTime", o, r, i)
}

func (s *stubTracking) DeleteIssueTimeEntry(_ context.Context, o, r string, i, id int64) error {
	return s.rec("DeleteIssueTimeEntry", o, r, i, id)
}

func (s *stubTracking) StartIssueStopwatch(_ context.Context, o, r string, i int64) error {
	return s.rec("StartIssueStopwatch", o, r, i)
}

func (s *stubTracking) StopIssueStopwatch(_ context.Context, o, r string, i int64) error {
	return s.rec("StopIssueStopwatch", o, r, i)
}

func (s *stubTracking) CancelIssueStopwatch(_ context.Context, o, r string, i int64) error {
	return s.rec("CancelIssueStopwatch", o, r, i)
}

func (s *stubTracking) ListMyStopwatches(context.Context) ([]*forgejo_sdk.StopWatch, error) {
	return []*forgejo_sdk.StopWatch{{Created: t0, Seconds: 90, Duration: "1m30s", IssueIndex: 7, IssueTitle: "fixed", RepoOwnerName: "octo", RepoName: "hello"}}, s.rec("ListMyStopwatches")
}

// invoke calls a registered tool the way the server does and renders what the
// client would see: the text, or the error, then the port calls made. A
// non-nil credErr makes the credential policy refuse the request.
func invoke(t *testing.T, stub *stubTracking, credErr error, tool string, args map[string]any) string {
	t.Helper()
	s := server.NewMCPServer("test", "0")
	RegisterTool(s, fake.Factory{Forgejo: stub, Err: credErr})
	st := s.GetTool(tool)
	if st == nil {
		t.Fatalf("tool %s not registered", tool)
	}
	base := map[string]any{"owner": "octo", "repo": "hello"}
	for k, v := range args {
		if v == nil {
			delete(base, k) // a nil value means "leave the argument out"
			continue
		}
		base[k] = v
	}
	res, err := st.Handler(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Name: tool, Arguments: base}})
	var out string
	switch {
	case err != nil:
		out = "ERROR: " + err.Error()
	case res.IsError:
		out = "RESULT-ERROR: " + res.Content[0].(mcp.TextContent).Text
	default:
		out = res.Content[0].(mcp.TextContent).Text
	}
	return fmt.Sprintf("%s\n-- port calls --\n%s\n", out, strings.Join(stub.calls, "\n"))
}

type goldenCase struct {
	name    string
	tool    string
	args    map[string]any
	stub    func(*stubTracking)
	credErr error
}

func failPort(s *stubTracking) { s.fail = errors.New("boom") }

var goldenCases = []goldenCase{
	// list_issue_tracked_times
	{name: "issue_times_defaults", tool: ListIssueTrackedTimesToolName, args: map[string]any{"index": float64(7)}},
	{name: "issue_times_filtered", tool: ListIssueTrackedTimesToolName, args: map[string]any{"index": float64(7), "since": "2026-01-01T00:00:00Z", "before": "2026-02-01T00:00:00Z", "page": float64(2), "limit": float64(5)}},
	{name: "issue_times_fractional", tool: ListIssueTrackedTimesToolName, args: map[string]any{"index": 7.9, "page": 1.5, "limit": 2.5}},
	{name: "issue_times_string_numbers", tool: ListIssueTrackedTimesToolName, args: map[string]any{"index": "7", "page": "3", "limit": "4"}},
	{name: "issue_times_bad_since", tool: ListIssueTrackedTimesToolName, args: map[string]any{"index": float64(7), "since": "yesterday"}},
	{name: "issue_times_bad_before", tool: ListIssueTrackedTimesToolName, args: map[string]any{"index": float64(7), "before": "2026-02-01"}},
	{name: "issue_times_non_string_since", tool: ListIssueTrackedTimesToolName, args: map[string]any{"index": float64(7), "since": float64(5)}},
	{name: "issue_times_bad_since_no_credential", tool: ListIssueTrackedTimesToolName, args: map[string]any{"index": float64(7), "since": "yesterday"}, credErr: errors.New("no credential")},
	{name: "issue_times_no_credential", tool: ListIssueTrackedTimesToolName, args: map[string]any{"index": float64(7)}, credErr: errors.New("no credential")},
	{name: "issue_times_port_error", tool: ListIssueTrackedTimesToolName, args: map[string]any{"index": float64(7)}, stub: failPort},

	// list_repo_tracked_times
	{name: "repo_times_defaults", tool: ListRepoTrackedTimesToolName},
	{name: "repo_times_filtered", tool: ListRepoTrackedTimesToolName, args: map[string]any{"since": "2026-01-01T00:00:00Z", "user": "octo", "page": float64(3), "limit": float64(9)}},
	{name: "repo_times_bad_before", tool: ListRepoTrackedTimesToolName, args: map[string]any{"before": "soon"}},
	{name: "repo_times_port_error", tool: ListRepoTrackedTimesToolName, stub: failPort},

	// list_my_tracked_times
	{name: "my_times_ok", tool: ListMyTrackedTimesToolName},
	{name: "my_times_port_error", tool: ListMyTrackedTimesToolName, stub: failPort},
	{name: "my_times_no_credential", tool: ListMyTrackedTimesToolName, credErr: errors.New("no credential")},

	// add_issue_time
	{name: "add_time_seconds", tool: AddIssueTimeToolName, args: map[string]any{"index": float64(7), "seconds": float64(90)}},
	{name: "add_time_duration", tool: AddIssueTimeToolName, args: map[string]any{"index": float64(7), "duration": "1h30m"}},
	{name: "add_time_full", tool: AddIssueTimeToolName, args: map[string]any{"index": float64(7), "seconds": nil, "duration": "2m", "created_at": "2026-01-02T03:04:05Z", "user_name": "bob"}},
	{name: "add_time_seconds_fraction", tool: AddIssueTimeToolName, args: map[string]any{"index": float64(7), "seconds": 2.7}},
	{name: "add_time_seconds_string", tool: AddIssueTimeToolName, args: map[string]any{"index": float64(7), "seconds": "90"}},
	{name: "add_time_seconds_empty_string", tool: AddIssueTimeToolName, args: map[string]any{"index": float64(7), "seconds": "", "duration": "1s"}},
	{name: "add_time_seconds_malformed_string", tool: AddIssueTimeToolName, args: map[string]any{"index": float64(7), "seconds": "soon"}},
	{name: "add_time_seconds_bool", tool: AddIssueTimeToolName, args: map[string]any{"index": float64(7), "seconds": true}},
	{name: "add_time_both", tool: AddIssueTimeToolName, args: map[string]any{"index": float64(7), "seconds": float64(5), "duration": "1m"}},
	{name: "add_time_neither", tool: AddIssueTimeToolName, args: map[string]any{"index": float64(7)}},
	{name: "add_time_zero_seconds", tool: AddIssueTimeToolName, args: map[string]any{"index": float64(7), "seconds": float64(0)}},
	{name: "add_time_negative_seconds", tool: AddIssueTimeToolName, args: map[string]any{"index": float64(7), "seconds": float64(-5)}},
	{name: "add_time_bad_duration", tool: AddIssueTimeToolName, args: map[string]any{"index": float64(7), "duration": "bogus"}},
	{name: "add_time_zero_duration", tool: AddIssueTimeToolName, args: map[string]any{"index": float64(7), "duration": "0s"}},
	{name: "add_time_bad_created_at", tool: AddIssueTimeToolName, args: map[string]any{"index": float64(7), "seconds": float64(5), "created_at": "tomorrow"}},
	{name: "add_time_bad_created_at_no_credential", tool: AddIssueTimeToolName, args: map[string]any{"index": float64(7), "seconds": float64(5), "created_at": "tomorrow"}, credErr: errors.New("no credential")},
	{name: "add_time_port_error", tool: AddIssueTimeToolName, args: map[string]any{"index": float64(7), "seconds": float64(5)}, stub: failPort},

	// reset_issue_time, delete_issue_time_entry
	{name: "reset_time_ok", tool: ResetIssueTimeToolName, args: map[string]any{"index": float64(7)}},
	{name: "reset_time_port_error", tool: ResetIssueTimeToolName, args: map[string]any{"index": float64(7)}, stub: failPort},
	{name: "delete_time_entry_ok", tool: DeleteIssueTimeEntryToolName, args: map[string]any{"index": float64(7), "time_id": float64(5)}},
	{name: "delete_time_entry_string_id", tool: DeleteIssueTimeEntryToolName, args: map[string]any{"index": float64(7), "time_id": "5"}},
	{name: "delete_time_entry_port_error", tool: DeleteIssueTimeEntryToolName, args: map[string]any{"index": float64(7), "time_id": float64(5)}, stub: failPort},

	// stopwatches
	{name: "start_stopwatch_ok", tool: StartIssueStopwatchToolName, args: map[string]any{"index": float64(7)}},
	{name: "start_stopwatch_port_error", tool: StartIssueStopwatchToolName, args: map[string]any{"index": float64(7)}, stub: failPort},
	{name: "stop_stopwatch_ok", tool: StopIssueStopwatchToolName, args: map[string]any{"index": float64(7)}},
	{name: "stop_stopwatch_port_error", tool: StopIssueStopwatchToolName, args: map[string]any{"index": float64(7)}, stub: failPort},
	{name: "cancel_stopwatch_ok", tool: CancelIssueStopwatchToolName, args: map[string]any{"index": float64(7)}},
	{name: "cancel_stopwatch_port_error", tool: CancelIssueStopwatchToolName, args: map[string]any{"index": float64(7)}, stub: failPort},
	{name: "my_stopwatches_ok", tool: ListMyStopwatchesToolName},
	{name: "my_stopwatches_port_error", tool: ListMyStopwatchesToolName, stub: failPort},
}

func TestGolden(t *testing.T) {
	t.Parallel()
	for _, c := range goldenCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			stub := &stubTracking{}
			if c.stub != nil {
				c.stub(stub)
			}
			mcptest.Golden(t, c.name, []byte(invoke(t, stub, c.credErr, c.tool, c.args)))
		})
	}
}

// MaxResponseItems reports an unknown ceiling: the tracked-time goldens pin the
// paging contract without a ceiling, as they always have.
func (s *stubTracking) MaxResponseItems(context.Context) (int, bool) { return 0, false }
