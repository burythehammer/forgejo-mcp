// SPDX-License-Identifier: GPL-3.0-or-later

package tracking

import (
	"context"
	"strings"
	"testing"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/fake"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

// Handler tests run against the in-memory fake, build their own, and so run in
// parallel with no shared state.

const (
	testOwner = "goern"
	testRepo  = "forgejo-mcp"
)

// newFixture returns handlers served by a fresh fake holding issue #106.
func newFixture(t *testing.T) (handlers, *fake.Forgejo) {
	t.Helper()
	f := fake.New()
	f.AddTrackableIssue(testOwner, testRepo, 106, "track me")
	return handlers{forge: fake.Factory{Forgejo: f}}, f
}

func newCallToolRequest(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: args}}
}

func issueArgs(extra map[string]any) map[string]any {
	args := map[string]any{"owner": testOwner, "repo": testRepo, "index": 106.0}
	for k, v := range extra {
		args[k] = v
	}
	return args
}

type toolFn func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)

func textOf(res *mcp.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

// call runs a tool and fails the test unless it succeeded.
func call(t *testing.T, fn toolFn, args map[string]any) string {
	t.Helper()
	res, err := fn(context.Background(), newCallToolRequest(args))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("err=%v res=%+v", err, res)
	}
	return textOf(res)
}

// callErr runs a tool and returns its error, failing the test if it succeeded.
func callErr(t *testing.T, fn toolFn, args map[string]any) error {
	t.Helper()
	_, err := fn(context.Background(), newCallToolRequest(args))
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	return err
}

func tracked(t *testing.T, f *fake.Forgejo, index int64) []*forgejo_sdk.TrackedTime {
	t.Helper()
	got, err := f.ListIssueTrackedTimes(context.Background(), testOwner, testRepo, index, forgejo_sdk.ListTrackedTimesOptions{})
	if err != nil {
		t.Fatalf("reading tracked times: %v", err)
	}
	return got
}

// --- resolveSeconds ----------------------------------------------------------

func TestResolveSeconds_Table(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    map[string]any
		want    int64
		wantErr string
	}{
		{name: "seconds_only", args: map[string]any{"seconds": 900.0}, want: 900},
		{name: "duration_only_15m", args: map[string]any{"duration": "15m"}, want: 900},
		{name: "duration_1h30m", args: map[string]any{"duration": "1h30m"}, want: 5400},
		{name: "both_rejected", args: map[string]any{"seconds": 60.0, "duration": "1m"}, wantErr: "exactly one"},
		{name: "neither_rejected", args: map[string]any{}, wantErr: "required"},
		{name: "negative_seconds", args: map[string]any{"seconds": -5.0}, wantErr: "positive"},
		{name: "zero_seconds", args: map[string]any{"seconds": 0.0}, wantErr: "positive"},
		{name: "negative_duration", args: map[string]any{"duration": "-30s"}, wantErr: "positive"},
		{name: "malformed_duration", args: map[string]any{"duration": "not a duration"}, wantErr: "invalid duration"},
		{name: "empty_duration_treated_as_absent", args: map[string]any{"duration": ""}, wantErr: "required"},
		{name: "string_seconds_parsed", args: map[string]any{"seconds": "120"}, want: 120},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := resolveSeconds(tc.args)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %d seconds, want %d", got, tc.want)
			}
		})
	}
}

// --- AddIssueTimeFn ---------------------------------------------------------

func TestAddIssueTimeFn_DurationInput(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)

	call(t, h.AddIssueTimeFn, issueArgs(map[string]any{"duration": "15m"}))

	got := tracked(t, f, 106)
	if len(got) != 1 || got[0].Time != 900 {
		t.Fatalf("tracked times = %+v, want one entry of 900s", got)
	}
}

func TestAddIssueTimeFn_InvalidDuration_ReturnsStructuredError(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)

	err := callErr(t, h.AddIssueTimeFn, issueArgs(map[string]any{"duration": "bananas"}))

	if !strings.Contains(err.Error(), "invalid duration") {
		t.Errorf("unexpected error: %v", err)
	}
	if got := tracked(t, f, 106); len(got) != 0 {
		t.Errorf("validation failure must not log time, got %+v", got)
	}
}

func TestAddIssueTimeFn_CreatedAtRFC3339(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)

	call(t, h.AddIssueTimeFn, issueArgs(map[string]any{"seconds": 60.0, "created_at": "2026-04-21T20:00:00Z"}))

	got := tracked(t, f, 106)
	want := time.Date(2026, 4, 21, 20, 0, 0, 0, time.UTC)
	if len(got) != 1 || !got[0].Created.Equal(want) {
		t.Errorf("tracked times = %+v, want one entry created %s", got, want)
	}
}

func TestAddIssueTimeFn_UserName(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)

	call(t, h.AddIssueTimeFn, issueArgs(map[string]any{"seconds": 60.0, "user_name": "alice"}))

	if got := tracked(t, f, 106); len(got) != 1 || got[0].UserName != "alice" {
		t.Errorf("tracked times = %+v, want one entry for alice", got)
	}
}

func TestAddIssueTimeFn_InvalidCreatedAt(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)

	err := callErr(t, h.AddIssueTimeFn, issueArgs(map[string]any{"seconds": 60.0, "created_at": "yesterday afternoon"}))

	if !strings.Contains(err.Error(), "invalid created_at") {
		t.Fatalf("expected invalid created_at error, got %v", err)
	}
}

func TestAddIssueTimeFn_UnknownIssue(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)

	err := callErr(t, h.AddIssueTimeFn, issueArgs(map[string]any{"index": 999.0, "seconds": 60.0}))

	if !strings.Contains(err.Error(), "add issue time err") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- ListIssueTrackedTimesFn ------------------------------------------------

func TestListIssueTrackedTimesFn_FiltersApplied(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	for _, created := range []string{"2025-06-01T00:00:00Z", "2026-06-01T00:00:00Z", "2027-06-01T00:00:00Z"} {
		call(t, h.AddIssueTimeFn, issueArgs(map[string]any{"seconds": 60.0, "created_at": created}))
	}
	if got := tracked(t, f, 106); len(got) != 3 {
		t.Fatalf("seeded %d entries, want 3", len(got))
	}

	out := call(t, h.ListIssueTrackedTimesFn, issueArgs(map[string]any{
		"since": "2026-01-01T00:00:00Z", "before": "2026-12-31T23:59:59Z",
	}))

	if !strings.Contains(out, "2026-06-01") || strings.Contains(out, "2025-06-01") || strings.Contains(out, "2027-06-01") {
		t.Errorf("window not applied: %s", out)
	}
}

func TestListIssueTrackedTimesFn_Pagination(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	for _, secs := range []float64{11, 22, 33} {
		call(t, h.AddIssueTimeFn, issueArgs(map[string]any{"seconds": secs}))
	}

	out := call(t, h.ListIssueTrackedTimesFn, issueArgs(map[string]any{"page": 2.0, "limit": 2.0}))

	if !strings.Contains(out, `"time":33`) || strings.Contains(out, `"time":11`) || strings.Contains(out, `"time":22`) {
		t.Errorf("page 2 of 2-per-page should hold only the third entry: %s", out)
	}
}

func TestListIssueTrackedTimesFn_InvalidSince(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)

	callErr(t, h.ListIssueTrackedTimesFn, issueArgs(map[string]any{"since": "not-rfc3339"}))
}

// --- ListRepoTrackedTimesFn -------------------------------------------------

func TestListRepoTrackedTimesFn_UserFilterApplied(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	call(t, h.AddIssueTimeFn, issueArgs(map[string]any{"seconds": 61.0, "user_name": "alice"}))
	call(t, h.AddIssueTimeFn, issueArgs(map[string]any{"seconds": 62.0, "user_name": "bob"}))

	out := call(t, h.ListRepoTrackedTimesFn, map[string]any{"owner": testOwner, "repo": testRepo, "user": "alice"})

	if !strings.Contains(out, "alice") || strings.Contains(out, "bob") {
		t.Errorf("user filter not applied: %s", out)
	}
}

// --- ResetIssueTimeFn / DeleteIssueTimeEntryFn ------------------------------

func TestResetIssueTimeFn(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	call(t, h.AddIssueTimeFn, issueArgs(map[string]any{"seconds": 30.0}))
	call(t, h.AddIssueTimeFn, issueArgs(map[string]any{"seconds": 40.0}))

	out := call(t, h.ResetIssueTimeFn, issueArgs(nil))

	if !strings.Contains(out, "Reset tracked time success") {
		t.Errorf("result = %q", out)
	}
	if got := tracked(t, f, 106); len(got) != 0 {
		t.Errorf("entries remain after reset: %+v", got)
	}
}

func TestDeleteIssueTimeEntryFn(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	call(t, h.AddIssueTimeFn, issueArgs(map[string]any{"seconds": 30.0}))
	call(t, h.AddIssueTimeFn, issueArgs(map[string]any{"seconds": 40.0}))
	first := tracked(t, f, 106)[0]

	out := call(t, h.DeleteIssueTimeEntryFn, issueArgs(map[string]any{"time_id": float64(first.ID)}))

	if !strings.Contains(out, "Delete time entry success") {
		t.Errorf("result = %q", out)
	}
	if got := tracked(t, f, 106); len(got) != 1 || got[0].ID == first.ID {
		t.Errorf("entry %d not the one removed: %+v", first.ID, got)
	}
}

func TestDeleteIssueTimeEntryFn_UnknownEntry(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)

	err := callErr(t, h.DeleteIssueTimeEntryFn, issueArgs(map[string]any{"time_id": 12345.0}))

	if !strings.Contains(err.Error(), "delete time entry err") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Stopwatches ------------------------------------------------------------

func TestStartStopStopwatchRecordsTime(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	now := time.Date(2026, 5, 1, 9, 0, 0, 0, time.UTC)
	f.Now = func() time.Time { return now }

	if out := call(t, h.StartIssueStopwatchFn, issueArgs(nil)); !strings.Contains(out, "Stopwatch started") {
		t.Errorf("start result = %q", out)
	}
	now = now.Add(90 * time.Second)
	if out := call(t, h.StopIssueStopwatchFn, issueArgs(nil)); !strings.Contains(out, "Stopwatch stopped") {
		t.Errorf("stop result = %q", out)
	}

	got := tracked(t, f, 106)
	if len(got) != 1 || got[0].Time != 90 {
		t.Errorf("tracked times = %+v, want one entry of 90s", got)
	}
}

func TestCancelStopwatchRecordsNothing(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	call(t, h.StartIssueStopwatchFn, issueArgs(nil))

	if out := call(t, h.CancelIssueStopwatchFn, issueArgs(nil)); !strings.Contains(out, "Stopwatch cancelled") {
		t.Errorf("cancel result = %q", out)
	}

	if got := tracked(t, f, 106); len(got) != 0 {
		t.Errorf("cancel must not record time, got %+v", got)
	}
	if out := call(t, h.ListMyStopwatchesFn, nil); strings.Contains(out, "track me") {
		t.Errorf("cancelled stopwatch still listed: %s", out)
	}
}

func TestStopWithoutStartIsAnError(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)

	err := callErr(t, h.StopIssueStopwatchFn, issueArgs(nil))

	if !strings.Contains(err.Error(), "stop issue stopwatch err") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestListMyStopwatchesFn(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	call(t, h.StartIssueStopwatchFn, issueArgs(nil))

	out := call(t, h.ListMyStopwatchesFn, nil)

	if !strings.Contains(out, "track me") || !strings.Contains(out, testRepo) {
		t.Errorf("running stopwatch not listed: %s", out)
	}
}

// --- ListMyTrackedTimesFn ---------------------------------------------------

func TestListMyTrackedTimesFn(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	call(t, h.AddIssueTimeFn, issueArgs(map[string]any{"seconds": 77.0}))
	call(t, h.AddIssueTimeFn, issueArgs(map[string]any{"seconds": 88.0, "user_name": "someone-else"}))

	out := call(t, h.ListMyTrackedTimesFn, nil)

	if !strings.Contains(out, `"time":77`) || strings.Contains(out, `"time":88`) {
		t.Errorf("should list only the caller's entries: %s", out)
	}
}
