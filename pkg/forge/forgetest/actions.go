// SPDX-License-Identifier: GPL-3.0-or-later

package forgetest

import (
	"fmt"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// RunActions runs the Actions contract against the implementation that
// newHarness builds, once per case.
func RunActions(t *testing.T, newHarness func(t *testing.T) Harness) {
	cases := []struct {
		name string
		run  func(t *testing.T, h Harness)
	}{
		{"actions url names the repository", func(t *testing.T, h Harness) {
			if u := h.Port.ActionsURL(Owner, Repo); !strings.HasSuffix(u, "/"+Owner+"/"+Repo+"/actions") {
				t.Fatalf("got %q", u)
			}
		}},
		{"dispatch queues a run that lists back", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			h.Seed.AddWorkflow(Owner, Repo, "ci.yml")
			must(t, h.Port.DispatchWorkflow(ctx, Owner, Repo, "ci.yml", forgejo_sdk.DispatchWorkflowOption{Ref: "main"}))
			runs, err := h.Port.ListWorkflowRuns(ctx, Owner, Repo, forgejo_sdk.ListActionRunsOption{})
			must(t, err)
			if runs.TotalCount != 1 || len(runs.WorkflowRuns) != 1 || runs.WorkflowRuns[0].Event != "workflow_dispatch" {
				t.Fatalf("got %+v", runs)
			}
		}},
		{"dispatch of an unknown workflow or repository is not found", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			wantNotFound(t, h.Port.DispatchWorkflow(ctx, Owner, Repo, "nope.yml", forgejo_sdk.DispatchWorkflowOption{Ref: "main"}))
			wantNotFound(t, h.Port.DispatchWorkflow(ctx, "nobody", "nothing", "ci.yml", forgejo_sdk.DispatchWorkflowOption{Ref: "main"}))
		}},
		{"runs list newest first, filter and paginate", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			a := h.Seed.AddWorkflowRun(Owner, Repo, forgejo_sdk.ActionRun{Title: "a", Status: "success", Event: "push", CommitSHA: "aaa"})
			b := h.Seed.AddWorkflowRun(Owner, Repo, forgejo_sdk.ActionRun{Title: "b", Status: "failure", Event: "push", CommitSHA: "bbb"})
			c := h.Seed.AddWorkflowRun(Owner, Repo, forgejo_sdk.ActionRun{Title: "c", Status: "success", Event: "schedule", CommitSHA: "ccc"})

			all, err := h.Port.ListWorkflowRuns(ctx, Owner, Repo, forgejo_sdk.ListActionRunsOption{})
			must(t, err)
			wantRunIDs(t, all, c.ID, b.ID, a.ID)

			ok, err := h.Port.ListWorkflowRuns(ctx, Owner, Repo, forgejo_sdk.ListActionRunsOption{Status: "success"})
			must(t, err)
			wantRunIDs(t, ok, c.ID, a.ID)

			push, err := h.Port.ListWorkflowRuns(ctx, Owner, Repo, forgejo_sdk.ListActionRunsOption{Event: "push", HeadSHA: "bbb"})
			must(t, err)
			wantRunIDs(t, push, b.ID)

			second, err := h.Port.ListWorkflowRuns(ctx, Owner, Repo, forgejo_sdk.ListActionRunsOption{ListOptions: forgejo_sdk.ListOptions{Page: 2, PageSize: 2}})
			must(t, err)
			wantRunIDs(t, second, a.ID)
			if second.TotalCount != 3 {
				t.Fatalf("total %d, want 3", second.TotalCount)
			}
		}},
		{"get run reads back and unknown is not found", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			run := h.Seed.AddWorkflowRun(Owner, Repo, forgejo_sdk.ActionRun{Title: "build", Status: "success", Event: "push"})
			got, err := h.Port.GetWorkflowRun(ctx, Owner, Repo, run.ID)
			must(t, err)
			if got.ID != run.ID || got.Title != "build" || got.Status != "success" {
				t.Fatalf("got %+v", got)
			}
			_, err = h.Port.GetWorkflowRun(ctx, Owner, Repo, 999)
			wantNotFound(t, err)
		}},
		{"cancel stops a live run and leaves a finished one", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			live := h.Seed.AddWorkflowRun(Owner, Repo, forgejo_sdk.ActionRun{Status: "running"})
			done := h.Seed.AddWorkflowRun(Owner, Repo, forgejo_sdk.ActionRun{Status: "success"})
			must(t, h.Port.CancelWorkflowRun(ctx, Owner, Repo, live.ID))
			must(t, h.Port.CancelWorkflowRun(ctx, Owner, Repo, done.ID))
			if got, _ := h.Port.GetWorkflowRun(ctx, Owner, Repo, live.ID); got.Status != "cancelled" {
				t.Fatalf("live run is %q", got.Status)
			}
			if got, _ := h.Port.GetWorkflowRun(ctx, Owner, Repo, done.ID); got.Status != "success" {
				t.Fatalf("finished run is %q", got.Status)
			}
			wantNotFound(t, h.Port.CancelWorkflowRun(ctx, Owner, Repo, 999))
		}},
		{"delete removes a finished run and refuses a live one", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			live := h.Seed.AddWorkflowRun(Owner, Repo, forgejo_sdk.ActionRun{Status: "running"})
			done := h.Seed.AddWorkflowRun(Owner, Repo, forgejo_sdk.ActionRun{Status: "success"})
			if err := h.Port.DeleteWorkflowRun(ctx, Owner, Repo, live.ID); err == nil {
				t.Fatal("deleting a live run must fail")
			}
			must(t, h.Port.DeleteWorkflowRun(ctx, Owner, Repo, done.ID))
			_, err := h.Port.GetWorkflowRun(ctx, Owner, Repo, done.ID)
			wantNotFound(t, err)
			wantNotFound(t, h.Port.DeleteWorkflowRun(ctx, Owner, Repo, done.ID))
		}},
		{"jobs list for a run", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			run := h.Seed.AddWorkflowRun(Owner, Repo, forgejo_sdk.ActionRun{Status: "success"})
			empty, err := h.Port.ListRunJobs(ctx, Owner, Repo, run.ID)
			must(t, err)
			if len(empty) != 0 {
				t.Fatalf("got %+v", empty)
			}
			_, err = h.Seed.AddRunJob(Owner, Repo, run.ID, forge.ActionRunJob{Name: "build", Status: "success"})
			must(t, err)
			_, err = h.Seed.AddRunJob(Owner, Repo, run.ID, forge.ActionRunJob{Name: "test", Status: "failure"})
			must(t, err)
			jobs, err := h.Port.ListRunJobs(ctx, Owner, Repo, run.ID)
			must(t, err)
			if len(jobs) != 2 || jobs[0].Name != "build" || jobs[1].Name != "test" || jobs[0].RunID != run.ID {
				t.Fatalf("got %+v", jobs)
			}
			_, err = h.Port.ListRunJobs(ctx, Owner, Repo, 999)
			wantNotFound(t, err)
		}},
		{"job log reads the tail, a range and the whole log", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			run := h.Seed.AddWorkflowRun(Owner, Repo, forgejo_sdk.ActionRun{Status: "success"})
			job, err := h.Seed.AddRunJob(Owner, Repo, run.ID, forge.ActionRunJob{Name: "build"})
			must(t, err)
			must(t, h.Seed.SetJobLog(Owner, Repo, job.ID, "0123456789"))

			tail, err := h.Port.GetJobLog(ctx, Owner, Repo, job.ID, 0, "bytes=-4", 4)
			must(t, err)
			if string(tail.Body) != "6789" || tail.StatusCode != 206 || tail.ContentRange != "bytes 6-9/10" {
				t.Fatalf("tail: %q %d %q", tail.Body, tail.StatusCode, tail.ContentRange)
			}
			mid, err := h.Port.GetJobLog(ctx, Owner, Repo, job.ID, 0, "bytes=2-4", 3)
			must(t, err)
			if string(mid.Body) != "234" || mid.ContentRange != "bytes 2-4/10" {
				t.Fatalf("range: %q %q", mid.Body, mid.ContentRange)
			}
			whole, err := h.Port.GetJobLog(ctx, Owner, Repo, job.ID, 0, "", 100)
			must(t, err)
			if string(whole.Body) != "0123456789" || whole.StatusCode != 200 || whole.ContentLength != 10 {
				t.Fatalf("whole: %q %d %d", whole.Body, whole.StatusCode, whole.ContentLength)
			}
			if _, err := h.Port.GetJobLog(ctx, Owner, Repo, job.ID, 0, "", 4); err == nil {
				t.Fatal("a log over the cap must fail")
			}
		}},
		{"job log of an unknown job or attempt is not found", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			run := h.Seed.AddWorkflowRun(Owner, Repo, forgejo_sdk.ActionRun{Status: "success"})
			job, err := h.Seed.AddRunJob(Owner, Repo, run.ID, forge.ActionRunJob{Name: "build"})
			must(t, err)
			_, err = h.Port.GetJobLog(ctx, Owner, Repo, 999, 0, "", 10)
			wantNotFound(t, err)
			_, err = h.Port.GetJobLog(ctx, Owner, Repo, job.ID, 7, "", 10)
			wantNotFound(t, err)
		}},
		{"artifacts list with name filter, paging and total", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			run := h.Seed.AddWorkflowRun(Owner, Repo, forgejo_sdk.ActionRun{Status: "success"})
			for _, name := range []string{"dist", "logs", "dist"} {
				_, err := h.Seed.AddRunArtifact(Owner, Repo, run.ID, forge.ActionArtifact{Name: name, SizeInBytes: 5})
				must(t, err)
			}
			all, err := h.Port.ListRunArtifacts(ctx, Owner, Repo, run.ID, forge.ArtifactListOptions{Page: 1, Limit: 10})
			must(t, err)
			if len(all.Artifacts) != 3 || all.TotalCount == nil || *all.TotalCount != 3 {
				t.Fatalf("got %+v", all)
			}
			dist, err := h.Port.ListRunArtifacts(ctx, Owner, Repo, run.ID, forge.ArtifactListOptions{Page: 1, Limit: 10, Name: "dist"})
			must(t, err)
			if len(dist.Artifacts) != 2 {
				t.Fatalf("got %+v", dist)
			}
			second, err := h.Port.ListRunArtifacts(ctx, Owner, Repo, run.ID, forge.ArtifactListOptions{Page: 2, Limit: 2})
			must(t, err)
			if len(second.Artifacts) != 1 || *second.TotalCount != 3 {
				t.Fatalf("got %+v", second)
			}
			_, err = h.Port.ListRunArtifacts(ctx, Owner, Repo, 999, forge.ArtifactListOptions{Page: 1, Limit: 10})
			wantNotFound(t, err)
		}},
		{"artifact reads back by id", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			run := h.Seed.AddWorkflowRun(Owner, Repo, forgejo_sdk.ActionRun{Status: "success"})
			art, err := h.Seed.AddRunArtifact(Owner, Repo, run.ID, forge.ActionArtifact{Name: "dist", SizeInBytes: 42})
			must(t, err)
			got, err := h.Port.GetArtifact(ctx, Owner, Repo, art.ID)
			must(t, err)
			if got.Name != "dist" || got.SizeInBytes != 42 || got.RunID != run.ID {
				t.Fatalf("got %+v", got)
			}
			_, err = h.Port.GetArtifact(ctx, Owner, Repo, 999)
			wantNotFound(t, err)
		}},
		{"repository secrets: set, overwrite, list without values, delete", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			must(t, h.Port.SetRepoActionSecret(ctx, Owner, Repo, forgejo_sdk.CreateSecretOption{Name: "TOKEN", Data: "one"}))
			must(t, h.Port.SetRepoActionSecret(ctx, Owner, Repo, forgejo_sdk.CreateSecretOption{Name: "TOKEN", Data: "two"}))
			must(t, h.Port.SetRepoActionSecret(ctx, Owner, Repo, forgejo_sdk.CreateSecretOption{Name: "ALPHA", Data: "x"}))
			got, err := h.Port.ListRepoActionSecrets(ctx, Owner, Repo, forgejo_sdk.ListOptions{})
			must(t, err)
			wantSecrets(t, got, "ALPHA", "TOKEN")
			page2, err := h.Port.ListRepoActionSecrets(ctx, Owner, Repo, forgejo_sdk.ListOptions{Page: 2, PageSize: 1})
			must(t, err)
			wantSecrets(t, page2, "TOKEN")
			must(t, h.Port.DeleteRepoActionSecret(ctx, Owner, Repo, "TOKEN"))
			got, err = h.Port.ListRepoActionSecrets(ctx, Owner, Repo, forgejo_sdk.ListOptions{})
			must(t, err)
			wantSecrets(t, got, "ALPHA")
			wantNotFound(t, h.Port.DeleteRepoActionSecret(ctx, Owner, Repo, "TOKEN"))
		}},
		{"repository secrets in an unknown repository are not found", func(t *testing.T, h Harness) {
			_, err := h.Port.ListRepoActionSecrets(ctx, "nobody", "nothing", forgejo_sdk.ListOptions{})
			wantNotFound(t, err)
			wantNotFound(t, h.Port.SetRepoActionSecret(ctx, "nobody", "nothing", forgejo_sdk.CreateSecretOption{Name: "A", Data: "b"}))
		}},
		{"a secret without data is rejected", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			if err := h.Port.SetRepoActionSecret(ctx, Owner, Repo, forgejo_sdk.CreateSecretOption{Name: "A"}); err == nil {
				t.Fatal("want an error")
			}
		}},
		{"organization secrets: set, list, delete", func(t *testing.T, h Harness) {
			h.Seed.AddActionsOrg("acme")
			must(t, h.Port.SetOrgActionSecret(ctx, "acme", forgejo_sdk.CreateSecretOption{Name: "DEPLOY", Data: "x"}))
			got, err := h.Port.ListOrgActionSecrets(ctx, "acme", forgejo_sdk.ListOptions{})
			must(t, err)
			wantSecrets(t, got, "DEPLOY")
			must(t, h.Port.DeleteOrgActionSecret(ctx, "acme", "DEPLOY"))
			wantNotFound(t, h.Port.DeleteOrgActionSecret(ctx, "acme", "DEPLOY"))
			_, err = h.Port.ListOrgActionSecrets(ctx, "ghost", forgejo_sdk.ListOptions{})
			wantNotFound(t, err)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.run(t, newHarness(t))
		})
	}
}

func wantRunIDs(t *testing.T, got *forgejo_sdk.ListActionRunsResponse, want ...int64) {
	t.Helper()
	have := make([]int64, len(got.WorkflowRuns))
	for i, r := range got.WorkflowRuns {
		have[i] = r.ID
	}
	if fmt.Sprint(have) != fmt.Sprint(append([]int64{}, want...)) {
		t.Fatalf("got run IDs %v, want %v", have, want)
	}
}

// wantSecrets checks the names listed, and that no value came back.
func wantSecrets(t *testing.T, got []*forgejo_sdk.Secret, want ...string) {
	t.Helper()
	have := make([]string, len(got))
	for i, s := range got {
		have[i] = s.Name
		if s.Data != "" {
			t.Fatalf("secret %s leaked its value", s.Name)
		}
	}
	if fmt.Sprint(have) != fmt.Sprint(append([]string{}, want...)) {
		t.Fatalf("got secrets %v, want %v", have, want)
	}
}
