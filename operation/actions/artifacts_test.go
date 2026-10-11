// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/fake"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

func runWithArtifacts(t *testing.T, f *fake.Forgejo, names ...string) int64 {
	t.Helper()
	run := f.AddWorkflowRun(testOwner, testRepo, forgejo_sdk.ActionRun{Status: "success"})
	for _, n := range names {
		if _, err := f.AddRunArtifact(testOwner, testRepo, run.ID, forge.ActionArtifact{Name: n, SizeInBytes: 10}); err != nil {
			t.Fatalf("seeding artifact: %v", err)
		}
	}
	return run.ID
}

func TestListActionRunArtifactsFn_ServerPagedEnvelope(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	runID := runWithArtifacts(t, f, "dist", "logs", "dist", "other")

	got := decode[listActionRunArtifactsResult](t, h.ListActionRunArtifactsFn, repoArgs(map[string]any{
		"run_id": float64(runID), "page": 2.0, "limit": 1.0, "name": "dist",
	}))

	if got.RunID != runID || got.Page != 2 || got.Limit != 1 || got.Count != 1 {
		t.Fatalf("envelope: %+v", got)
	}
	if got.TotalCount == nil || *got.TotalCount != 2 {
		t.Fatalf("total_count should be the filtered grand total: %+v", got.TotalCount)
	}
	if len(got.Artifacts) != 1 || got.Artifacts[0].Name != "dist" {
		t.Fatalf("artifacts: %+v", got.Artifacts)
	}
}

func TestListActionRunArtifactsFn_EmptyPageIsAnEmptyList(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	runID := runWithArtifacts(t, f)

	out := call(t, h.ListActionRunArtifactsFn, repoArgs(map[string]any{"run_id": float64(runID)}))
	got := decode[listActionRunArtifactsResult](t, h.ListActionRunArtifactsFn, repoArgs(map[string]any{"run_id": float64(runID)}))

	if got.Count != 0 || got.Artifacts == nil {
		t.Fatalf("empty page: %+v", got)
	}
	if !strings.Contains(out, `"artifacts":[]`) {
		t.Fatalf("artifacts must serialise as [], got %s", out)
	}
}

func TestListActionRunArtifactsFn_MissingRunIsError(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)

	err := callErr(t, h.ListActionRunArtifactsFn, repoArgs(map[string]any{"run_id": 99.0}))

	if !strings.Contains(err.Error(), "list action run artifacts") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGetActionArtifactFn_MetadataOnly(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	runID := runWithArtifacts(t, f, "dist")
	art, err := f.GetArtifact(t.Context(), testOwner, testRepo, 1)
	if err != nil {
		t.Fatal(err)
	}

	got := decode[forge.ActionArtifact](t, h.GetActionArtifactFn, repoArgs(map[string]any{"artifact_id": float64(art.ID)}))

	if got.ID != art.ID || got.Name != "dist" || got.RunID != runID || got.SizeInBytes != 10 {
		t.Fatalf("artifact: %+v", got)
	}
}

func TestGetActionArtifactFn_UnknownAndInvalid(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)

	err := callErr(t, h.GetActionArtifactFn, repoArgs(map[string]any{"artifact_id": 9.0}))
	if !strings.Contains(err.Error(), "get action artifact") {
		t.Errorf("unexpected error: %v", err)
	}
	callErr(t, h.GetActionArtifactFn, repoArgs(nil))
}
