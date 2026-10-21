// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	"math"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// runWithJob seeds a run holding one job whose log is log, and returns the job.
func runWithJob(t *testing.T, f *fake.Forgejo, log string) forge.ActionRunJob {
	t.Helper()
	run := f.AddWorkflowRun(testOwner, testRepo, forgejo_sdk.ActionRun{Status: "success"})
	job, err := f.AddRunJob(testOwner, testRepo, run.ID, forge.ActionRunJob{Name: "build"})
	if err != nil {
		t.Fatalf("seeding job: %v", err)
	}
	if err := f.SetJobLog(testOwner, testRepo, job.ID, log); err != nil {
		t.Fatalf("seeding log: %v", err)
	}
	return job
}

func TestListActionRunJobsFn_PaginatesJobs(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	run := f.AddWorkflowRun(testOwner, testRepo, forgejo_sdk.ActionRun{Status: "success"})
	for _, n := range []string{"lint", "test", "build"} {
		if _, err := f.AddRunJob(testOwner, testRepo, run.ID, forge.ActionRunJob{Name: n}); err != nil {
			t.Fatal(err)
		}
	}

	last := resultOf[model.JobPage](t, h.ListActionRunJobsFn, repoArgs(map[string]any{"run_id": float64(run.ID), "page": 2.0, "limit": 2.0}))
	if last.TotalCount == nil || *last.TotalCount != 3 || len(last.Jobs) != 1 || last.Jobs[0].Name != "build" || last.HasNext || last.NextPage != 0 {
		t.Fatalf("unexpected last page: %+v", last)
	}

	first := resultOf[model.JobPage](t, h.ListActionRunJobsFn, repoArgs(map[string]any{"run_id": float64(run.ID), "page": 1.0, "limit": 2.0}))
	if !first.HasNext || first.NextPage != 2 || len(first.Jobs) != 2 {
		t.Fatalf("unexpected continuation metadata: %+v", first)
	}
}

func TestListActionRunJobsFn_UnknownRun(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)

	err := callErr(t, h.ListActionRunJobsFn, repoArgs(map[string]any{"run_id": 77.0}))

	if !strings.Contains(err.Error(), "list action run jobs") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGetActionJobLogsFn_DefaultsToTail(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	job := runWithJob(t, f, strings.Repeat("a", 68)+strings.Repeat("x", 32))

	got := resultOf[model.JobLogPage](t, h.GetActionJobLogsFn, repoArgs(map[string]any{"job_id": float64(job.ID), "attempt": 1.0, "max_bytes": 32.0}))

	if got.Content != strings.Repeat("x", 32) || got.StartByte != 68 || got.EndByte != 99 || got.TotalBytes != 100 {
		t.Fatalf("unexpected window: %+v", got)
	}
	if !got.TruncatedBefore || got.TruncatedAfter || got.PreviousOffset == nil || *got.PreviousOffset != 36 {
		t.Fatalf("unexpected continuation metadata: %+v", got)
	}
	if got.PreviousMaxBytes == nil || *got.PreviousMaxBytes != 32 || got.BytesReturned != 32 || got.Attempt != 1 {
		t.Fatalf("unexpected result metadata: %+v", got)
	}
}

func TestGetActionJobLogsFn_OffsetProvidesNextChunk(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	job := runWithJob(t, f, "0123456789error0123456789012345")

	got := resultOf[model.JobLogPage](t, h.GetActionJobLogsFn, repoArgs(map[string]any{"job_id": float64(job.ID), "offset": 10.0, "max_bytes": 5.0}))

	if got.Content != "error" || got.NextOffset == nil || *got.NextOffset != 15 {
		t.Fatalf("unexpected result: %+v", got)
	}
	if got.PreviousOffset == nil || *got.PreviousOffset != 5 {
		t.Fatalf("missing previous offset: %+v", got)
	}
}

func TestGetActionJobLogsFn_BoundsShortPreviousChunkWithoutOverlap(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	job := runWithJob(t, f, strings.Repeat("x", 37))

	got := resultOf[model.JobLogPage](t, h.GetActionJobLogsFn, repoArgs(map[string]any{"job_id": float64(job.ID), "max_bytes": 32.0}))

	if got.PreviousOffset == nil || *got.PreviousOffset != 0 || got.PreviousMaxBytes == nil || *got.PreviousMaxBytes != 5 {
		t.Fatalf("previous chunk would overlap current content: %+v", got)
	}
}

func TestGetActionJobLogsFn_HandlesEmptyLog(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	job := runWithJob(t, f, "")

	got := resultOf[model.JobLogPage](t, h.GetActionJobLogsFn, repoArgs(map[string]any{"job_id": float64(job.ID)}))

	if got.Content != "" || got.StartByte != 0 || got.EndByte != -1 || got.TotalBytes != 0 {
		t.Fatalf("unexpected empty log metadata: %+v", got)
	}
	if got.TruncatedBefore || got.TruncatedAfter || got.PreviousOffset != nil || got.PreviousMaxBytes != nil || got.NextOffset != nil {
		t.Fatalf("empty log advertised continuation: %+v", got)
	}
}

func TestGetActionJobLogsFn_UnknownJob(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)

	err := callErr(t, h.GetActionJobLogsFn, repoArgs(map[string]any{"job_id": 5.0}))

	if !strings.Contains(err.Error(), "get action job logs") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGetActionJobLogsFn_RejectsInvalidBounds(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	for _, extra := range []map[string]any{
		{"job_id": 0.0},
		{"job_id": math.Pow(2, 63)},
		{"job_id": 1.0, "attempt": 1.5},
		{"job_id": 1.0, "offset": -1.0},
		{"job_id": 1.0, "offset": math.Pow(2, 63)},
		{"job_id": 1.0, "max_bytes": float64(maxActionLogBytes + 1)},
	} {
		callErr(t, h.GetActionJobLogsFn, repoArgs(extra))
	}
}
