// SPDX-License-Identifier: GPL-3.0-or-later

package actionsadapter_test

import (
	"context"
	"encoding/json"
	"testing"

	app "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/actions"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/actionsadapter"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/fake"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// roundTrip marshals from and unmarshals into to, then reports whether the JSON
// survived intact. A field the repo-owned copy lost or renamed shows up here.
func roundTrip(t *testing.T, from, to any) bool {
	t.Helper()
	a, err := json.Marshal(from)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(a, to); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(to)
	if err != nil {
		t.Fatal(err)
	}
	return string(a) == string(b)
}

// The repo-owned wire types must carry exactly the JSON the SDK-facing forge
// types carry, or a tool would silently drop a field.
func TestModelTypesKeepTheWireShape(t *testing.T) {
	t.Parallel()
	art := forge.ActionArtifact{ID: 1, Name: "a", RunID: 2, SizeInBytes: 3, Expired: true,
		ExpiresAt: "x", CreatedAt: "y", UpdatedAt: "z", ArchiveDownloadURL: "u"}
	var gotArt model.ActionArtifact
	if !roundTrip(t, art, &gotArt) {
		t.Errorf("model.ActionArtifact drops or renames JSON fields of forge.ActionArtifact")
	}
	job := forge.ActionRunJob{ID: 1, RunID: 2, Attempt: 3, Handle: "h", RepoID: 4, OwnerID: 5,
		Name: "n", Needs: []string{"a"}, RunsOn: []string{"b"}, TaskID: 6, Status: "s"}
	var gotJob model.ActionRunJob
	if !roundTrip(t, job, &gotJob) {
		t.Errorf("model.ActionRunJob drops or renames JSON fields of forge.ActionRunJob")
	}
}

func TestUseCaseThroughTheBridgeAndFake(t *testing.T) {
	t.Parallel()
	f := fake.New()
	f.AddRepo("o", "r")
	run := f.AddWorkflowRun("o", "r", forgejo_sdk.ActionRun{Title: "CI", Status: "success", Event: "push"})
	if _, err := f.AddRunJob("o", "r", run.ID, forge.ActionRunJob{Name: "build"}); err != nil {
		t.Fatal(err)
	}
	svc := app.New(actionsadapter.New(f))
	ctx := context.Background()

	list, err := svc.ListRuns(ctx, "o", "r", app.RunQuery{Paging: app.Paging{Page: 1, Limit: 30}})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.WorkflowRuns) != 1 || list.WorkflowRuns[0].ID != run.ID || list.WorkflowRuns[0].Title != "CI" {
		t.Fatalf("runs through the bridge: %+v", list)
	}
	jobs, err := svc.ListJobs(ctx, "o", "r", run.ID, app.Paging{Page: 1, Limit: 30})
	if err != nil {
		t.Fatal(err)
	}
	if jobs.TotalCount == nil || *jobs.TotalCount != 1 || jobs.Jobs[0].Name != "build" {
		t.Fatalf("jobs through the bridge: %+v", jobs)
	}
	if _, err := svc.GetRun(ctx, "o", "r", 9999); err == nil {
		t.Fatal("an unknown run must be an error, not an empty run")
	}
}
