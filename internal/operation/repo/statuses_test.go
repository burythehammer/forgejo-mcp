// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

func statusesReq(extra map[string]any) mcp.CallToolRequest {
	args := map[string]any{"sha": testSHA}
	for k, v := range extra {
		args[k] = v
	}
	return filesReq(args)
}

func decodeStatuses(t *testing.T, res *mcp.CallToolResult) model.CommitStatuses {
	t.Helper()
	var envelope struct {
		Result model.CommitStatuses `json:"Result"`
	}
	if err := json.Unmarshal([]byte(extractText(t, res)), &envelope); err != nil {
		t.Fatalf("result JSON: %v", err)
	}
	return envelope.Result
}

// knownCommit seeds a commit so its (empty) status list is not a 404.
func knownCommit(t *testing.T, f *fake.Forgejo) {
	t.Helper()
	if err := f.AddCommit(filesOwner, filesRepo, testSHA, "c"); err != nil {
		t.Fatal(err)
	}
}

func TestGetCommitStatusesFn_ShortSHA(t *testing.T) {
	t.Parallel()
	h, _ := newFilesFixture(t)
	_, err := h.GetCommitStatusesFn(context.Background(), filesReq(map[string]any{"sha": "abc123"}))
	if err == nil {
		t.Fatal("expected error for short sha")
	}
}

func TestGetCommitStatusesFn_EmptyList(t *testing.T) {
	t.Parallel()
	h, f := newFilesFixture(t)
	knownCommit(t, f)

	res, err := h.GetCommitStatusesFn(context.Background(), statusesReq(nil))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("list failed: err=%v res=%+v", err, res)
	}
	got := decodeStatuses(t, res)
	if got.SHA != testSHA || got.Count != 0 {
		t.Errorf("sha=%q count=%d", got.SHA, got.Count)
	}
	if got.Statuses == nil || len(got.Statuses) != 0 {
		t.Errorf("statuses must be [] not null, got %v", got.Statuses)
	}
	if got.Page != 1 || got.Limit != 30 {
		t.Errorf("page/limit: got page=%d limit=%d", got.Page, got.Limit)
	}
	if got.TotalCount == nil || *got.TotalCount != 0 {
		t.Errorf("a confirmed-zero total_count must be emitted, got %v", got.TotalCount)
	}
}

func TestGetCommitStatusesFn_SuccessPage(t *testing.T) {
	t.Parallel()
	h, f := newFilesFixture(t)
	seedStatuses(t, f, forgejo_sdk.StatusSuccess)

	res, err := h.GetCommitStatusesFn(context.Background(), statusesReq(nil))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("list failed: err=%v res=%+v", err, res)
	}
	got := decodeStatuses(t, res)
	if got.Count != 1 || len(got.Statuses) != 1 {
		t.Fatalf("count=%d statuses=%v", got.Count, got.Statuses)
	}
	item := got.Statuses[0]
	if item.Context != "ci/test-0" || item.State != "success" || item.Description != "test context success" || item.TargetURL == "" || item.CreatedAt == "" {
		t.Errorf("item: %+v", item)
	}
	text := extractText(t, res)
	if !strings.Contains(text, `"state":"success"`) || strings.Contains(text, `"status":"success"`) {
		t.Errorf("item must use state, not the SDK's status: %s", text)
	}
	if got.TotalCount == nil || *got.TotalCount != 1 {
		t.Errorf("total_count: %v", got.TotalCount)
	}
}

func TestGetCommitStatusesFn_UnknownCommitIsAToolError(t *testing.T) {
	t.Parallel()
	h, _ := newFilesFixture(t)
	res, err := h.GetCommitStatusesFn(context.Background(), statusesReq(nil))
	if err == nil && (res == nil || !res.IsError) {
		t.Fatalf("expected an error for an unknown commit, got %v", res)
	}
}

func TestGetCommitStatusesFn_Paginates(t *testing.T) {
	t.Parallel()
	h, f := newFilesFixture(t)
	seedStatuses(t, f, forgejo_sdk.StatusSuccess, forgejo_sdk.StatusFailure, forgejo_sdk.StatusPending)

	res, err := h.GetCommitStatusesFn(context.Background(), statusesReq(map[string]any{"page": float64(2), "limit": float64(2)}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("list failed: err=%v res=%+v", err, res)
	}
	got := decodeStatuses(t, res)
	if got.Page != 2 || got.Limit != 2 || got.Count != 1 || got.Statuses[0].Context != "ci/test-2" {
		t.Errorf("page 2 = %+v", got)
	}
	if got.TotalCount == nil || *got.TotalCount != 3 {
		t.Errorf("total_count: %v", got.TotalCount)
	}
}

func TestGetCommitStatusesFn_LimitClamp(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		limit float64
		echo  int
	}{
		{name: "over_max", limit: 999, echo: 50},
		{name: "under_min", limit: 0, echo: 30},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h, f := newFilesFixture(t)
			knownCommit(t, f)
			res, err := h.GetCommitStatusesFn(context.Background(), statusesReq(map[string]any{"limit": tc.limit}))
			if err != nil || res == nil || res.IsError {
				t.Fatalf("list failed: err=%v res=%+v", err, res)
			}
			if got := decodeStatuses(t, res); got.Limit != tc.echo {
				t.Errorf("limit: got %d, want %d", got.Limit, tc.echo)
			}
		})
	}
}

func TestGetCommitStatusesFn_LimitClampBoundsThePage(t *testing.T) {
	t.Parallel()
	h, f := newFilesFixture(t)
	states := make([]forgejo_sdk.StatusState, 60)
	for i := range states {
		states[i] = forgejo_sdk.StatusSuccess
	}
	seedStatuses(t, f, states...)

	res, err := h.GetCommitStatusesFn(context.Background(), statusesReq(map[string]any{"limit": float64(999)}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("list failed: err=%v res=%+v", err, res)
	}
	got := decodeStatuses(t, res)
	if got.Count != 50 || got.TotalCount == nil || *got.TotalCount != 60 {
		t.Errorf("count=%d total=%v, want 50 of 60", got.Count, got.TotalCount)
	}
}
