// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"strings"
	"testing"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

func listRuns(t *testing.T, f interface {
	ListWorkflowRuns(context.Context, string, string, forgejo_sdk.ListActionRunsOption) (*forgejo_sdk.ListActionRunsResponse, error)
},
) *forgejo_sdk.ListActionRunsResponse {
	t.Helper()
	runs, err := f.ListWorkflowRuns(context.Background(), testOwner, testRepo, forgejo_sdk.ListActionRunsOption{})
	if err != nil {
		t.Fatalf("listing runs: %v", err)
	}
	return runs
}

func TestDispatchWorkflowFn_Success(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	f.AddWorkflow(testOwner, testRepo, "ci.yml")

	out := call(t, h.DispatchWorkflowFn, repoArgs(map[string]any{"workflow": "ci.yml", "ref": "main"}))

	if !strings.Contains(out, "Workflow dispatched successfully") || !strings.Contains(out, "/o/r/actions") {
		t.Errorf("unexpected result: %s", out)
	}
	runs := listRuns(t, f)
	if len(runs.WorkflowRuns) != 1 || runs.WorkflowRuns[0].PrettyRef != "main" {
		t.Fatalf("dispatch should queue one run on main, got %+v", runs)
	}
}

func TestDispatchWorkflowFn_WithInputs(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	f.AddWorkflow(testOwner, testRepo, "deploy.yml")

	call(t, h.DispatchWorkflowFn, repoArgs(map[string]any{
		"workflow": "deploy.yml", "ref": "v1.0.0", "inputs": `{"environment": "production", "debug": "false"}`,
	}))

	if len(listRuns(t, f).WorkflowRuns) != 1 {
		t.Fatal("a valid inputs object must dispatch")
	}
}

func TestDispatchWorkflowFn_InvalidInputsJSON(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	f.AddWorkflow(testOwner, testRepo, "ci.yml")

	err := callErr(t, h.DispatchWorkflowFn, repoArgs(map[string]any{"workflow": "ci.yml", "ref": "main", "inputs": "not-valid-json"}))

	if !strings.Contains(err.Error(), "invalid inputs JSON") {
		t.Errorf("unexpected error: %v", err)
	}
	if len(listRuns(t, f).WorkflowRuns) != 0 {
		t.Error("nothing may be dispatched on invalid inputs")
	}
}

func TestDispatchWorkflowFn_UnknownWorkflow(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)

	err := callErr(t, h.DispatchWorkflowFn, repoArgs(map[string]any{"workflow": "nope.yml", "ref": "main"}))

	if !strings.Contains(err.Error(), "failed to dispatch workflow") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDispatchWorkflowFn_MissingArguments(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	for name, args := range map[string]map[string]any{
		"owner":    {"repo": testRepo, "workflow": "ci.yml", "ref": "main"},
		"workflow": {"owner": testOwner, "repo": testRepo, "ref": "main"},
		"ref":      {"owner": testOwner, "repo": testRepo, "workflow": "ci.yml"},
	} {
		if err := callErr(t, h.DispatchWorkflowFn, args); !strings.Contains(err.Error(), name+" is required") {
			t.Errorf("missing %s: %v", name, err)
		}
	}
}

func TestListWorkflowRunsFn_ReturnsRuns(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	f.AddWorkflowRun(testOwner, testRepo, forgejo_sdk.ActionRun{Title: "Run CI", Status: "success", Event: "push", CommitSHA: "abc1234567890"})
	f.AddWorkflowRun(testOwner, testRepo, forgejo_sdk.ActionRun{Title: "Deploy", Status: "running", Event: "workflow_dispatch", CommitSHA: "def9876543210"})

	out := call(t, h.ListWorkflowRunsFn, repoArgs(nil))

	for _, want := range []string{`"total_count":2`, `"count":2`, `"has_next":false`, "Run CI", "Deploy", `"commit_sha":"abc1234567890"`, `"commit_sha":"def9876543210"`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %s", want, out)
		}
	}
}

func TestListWorkflowRunsFn_Filters(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	f.AddWorkflowRun(testOwner, testRepo, forgejo_sdk.ActionRun{Title: "Run CI", Status: "success", Event: "push"})
	f.AddWorkflowRun(testOwner, testRepo, forgejo_sdk.ActionRun{Title: "Deploy", Status: "running", Event: "workflow_dispatch"})

	out := call(t, h.ListWorkflowRunsFn, repoArgs(map[string]any{"status": "running"}))

	if !strings.Contains(out, "Deploy") || strings.Contains(out, "Run CI") {
		t.Errorf("status filter not applied: %s", out)
	}
}

func TestListWorkflowRunsFn_EmptyResult(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)

	out := call(t, h.ListWorkflowRunsFn, repoArgs(nil))

	if !strings.Contains(out, `"workflow_runs":[]`) || !strings.Contains(out, `"has_next":false`) {
		t.Errorf("unexpected result: %s", out)
	}
}

func TestListWorkflowRunsFn_MissingRepo(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)

	callErr(t, h.ListWorkflowRunsFn, map[string]any{"owner": testOwner})
}

func TestGetWorkflowRunFn_ReturnsRun(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	run := f.AddWorkflowRun(testOwner, testRepo, forgejo_sdk.ActionRun{
		Title: "CI Pipeline", Status: "success", Event: "push", CommitSHA: "deadbeef", PrettyRef: "refs/heads/main",
		TriggerUser: &forgejo_sdk.User{UserName: "testuser"},
	})

	out := call(t, h.GetWorkflowRunFn, repoArgs(map[string]any{"run_id": float64(run.ID)}))

	for _, want := range []string{"CI Pipeline", "Status: success", "Triggered by: testuser", "Commit SHA: deadbeef"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %s", want, out)
		}
	}
}

func TestGetWorkflowRunFn_UnknownRun(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)

	err := callErr(t, h.GetWorkflowRunFn, repoArgs(map[string]any{"run_id": 99.0}))

	if !strings.Contains(err.Error(), "failed to get workflow run") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGetWorkflowRunFn_InvalidRunID(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)

	callErr(t, h.GetWorkflowRunFn, repoArgs(map[string]any{"run_id": -1.0}))
	callErr(t, h.GetWorkflowRunFn, repoArgs(nil))
}
