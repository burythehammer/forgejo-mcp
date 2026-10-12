// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"strings"
	"testing"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

func runStatus(t *testing.T, h handlers, id int64) string {
	t.Helper()
	port, _ := h.forge.For(context.Background())
	run, err := port.GetWorkflowRun(context.Background(), testOwner, testRepo, id)
	if err != nil {
		t.Fatalf("reading run %d: %v", id, err)
	}
	return run.Status
}

func TestCancelWorkflowRunFn_RunningRun(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	run := f.AddWorkflowRun(testOwner, testRepo, forgejo_sdk.ActionRun{Status: "running"})

	got := resultOf[workflowRunControlResult](t, h.CancelWorkflowRunFn, repoArgs(map[string]any{"run_id": float64(run.ID)}))

	if got.RunID != run.ID || got.Status != "cancelled" {
		t.Fatalf("result: %+v", got)
	}
	if s := runStatus(t, h, run.ID); s != "cancelled" {
		t.Fatalf("run is %q", s)
	}
}

func TestCancelWorkflowRunFn_AlreadyFinishedIsSuccess(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	run := f.AddWorkflowRun(testOwner, testRepo, forgejo_sdk.ActionRun{Status: "success"})

	got := resultOf[workflowRunControlResult](t, h.CancelWorkflowRunFn, repoArgs(map[string]any{"run_id": float64(run.ID)}))

	if got.Status != "cancelled" {
		t.Fatalf("result: %+v", got)
	}
	if s := runStatus(t, h, run.ID); s != "success" {
		t.Fatalf("a finished run must be left unchanged, is %q", s)
	}
}

func TestCancelWorkflowRunFn_UnknownRun(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)

	err := callErr(t, h.CancelWorkflowRunFn, repoArgs(map[string]any{"run_id": 5.0}))

	if !strings.Contains(err.Error(), "cancel workflow run") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDeleteWorkflowRunFn_CompletedRun(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	run := f.AddWorkflowRun(testOwner, testRepo, forgejo_sdk.ActionRun{Status: "failure"})

	got := resultOf[workflowRunControlResult](t, h.DeleteWorkflowRunFn, repoArgs(map[string]any{"run_id": float64(run.ID)}))

	if got.RunID != run.ID || got.Status != "deleted" {
		t.Fatalf("result: %+v", got)
	}
	callErr(t, h.GetWorkflowRunFn, repoArgs(map[string]any{"run_id": float64(run.ID)}))
}

func TestDeleteWorkflowRunFn_LiveRunIsError(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	run := f.AddWorkflowRun(testOwner, testRepo, forgejo_sdk.ActionRun{Status: "running"})

	err := callErr(t, h.DeleteWorkflowRunFn, repoArgs(map[string]any{"run_id": float64(run.ID)}))

	if !strings.Contains(err.Error(), "delete workflow run") || strings.Contains(err.Error(), `"status":"deleted"`) {
		t.Errorf("unexpected error: %v", err)
	}
	if s := runStatus(t, h, run.ID); s != "running" {
		t.Fatalf("live run must survive, is %q", s)
	}
}

func TestCancelWorkflowRunFn_InvalidArguments(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	for _, args := range []map[string]any{
		{"repo": "r", "run_id": 1.0},
		{"owner": "o", "run_id": 1.0},
		{"owner": "o", "repo": "r"},
		{"owner": "o", "repo": "r", "run_id": 0.0},
	} {
		callErr(t, h.CancelWorkflowRunFn, args)
	}
}
