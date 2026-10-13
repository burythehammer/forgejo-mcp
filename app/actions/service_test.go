// SPDX-License-Identifier: GPL-3.0-or-later

package actions_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	app "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/actions"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
)

// stubPort is an in-memory app.Port. Each field is what the matching method
// returns; err, when set, is returned by every method that can fail.
type stubPort struct {
	err           error
	jobs          []model.ActionRunJob
	log           model.JobLog
	window        app.LogWindow
	secrets       []model.Secret
	dispatch      app.DispatchInput
	gotSecret     [2]string
	runs          []model.WorkflowRun
	runTotal      int64
	artifacts     []model.ActionArtifact
	artifactTotal *int
}

// serve returns the rows of all that a server-paged endpoint sends for p.
func serve[T any](all []T, p app.Paging) []T {
	lo := (p.Page - 1) * p.Limit
	if lo < 0 || lo > len(all) {
		lo = len(all)
	}
	return all[lo:min(lo+p.Limit, len(all))]
}

func (s *stubPort) ActionsURL(o, r string) string { return "https://f/" + o + "/" + r + "/actions" }
func (s *stubPort) DispatchWorkflow(_ context.Context, _, _, _ string, in app.DispatchInput) error {
	s.dispatch = in
	return s.err
}
func (s *stubPort) ListWorkflowRuns(_ context.Context, _, _ string, q app.RunQuery) (model.WorkflowRunList, error) {
	return model.WorkflowRunList{TotalCount: s.runTotal, WorkflowRuns: serve(s.runs, q.Paging)}, s.err
}
func (s *stubPort) GetWorkflowRun(context.Context, string, string, int64) (model.WorkflowRun, error) {
	return model.WorkflowRun{}, s.err
}
func (s *stubPort) CancelWorkflowRun(context.Context, string, string, int64) error { return s.err }
func (s *stubPort) DeleteWorkflowRun(context.Context, string, string, int64) error { return s.err }
func (s *stubPort) ListRunJobs(context.Context, string, string, int64) ([]model.ActionRunJob, error) {
	return s.jobs, s.err
}
func (s *stubPort) GetJobLog(_ context.Context, _, _ string, _, _ int64, w app.LogWindow) (model.JobLog, error) {
	s.window = w
	return s.log, s.err
}
func (s *stubPort) ListRunArtifacts(_ context.Context, _, _ string, _ int64, q app.ArtifactQuery) (model.ArtifactList, error) {
	return model.ArtifactList{Artifacts: serve(s.artifacts, q.Paging), TotalCount: s.artifactTotal}, s.err
}
func (s *stubPort) GetArtifact(context.Context, string, string, int64) (model.ActionArtifact, error) {
	return model.ActionArtifact{}, s.err
}
func (s *stubPort) ListRepoSecrets(_ context.Context, _, _ string, p app.Paging) ([]model.Secret, error) {
	return serve(s.secrets, p), s.err
}
func (s *stubPort) SetRepoSecret(_ context.Context, _, _, name, data string) error {
	s.gotSecret = [2]string{name, data}
	return s.err
}
func (s *stubPort) DeleteRepoSecret(context.Context, string, string, string) error { return s.err }
func (s *stubPort) ListOrgSecrets(_ context.Context, _ string, p app.Paging) ([]model.Secret, error) {
	return serve(s.secrets, p), s.err
}
func (s *stubPort) SetOrgSecret(context.Context, string, string, string) error { return s.err }
func (s *stubPort) DeleteOrgSecret(context.Context, string, string) error      { return s.err }

func TestDispatchReturnsActionsURL(t *testing.T) {
	t.Parallel()
	p := &stubPort{}
	url, err := app.New(p).Dispatch(context.Background(), "o", "r", "ci.yaml", app.DispatchInput{Ref: "main"})
	if err != nil {
		t.Fatal(err)
	}
	if url != "https://f/o/r/actions" || p.dispatch.Ref != "main" {
		t.Fatalf("url %q, input %+v", url, p.dispatch)
	}
}

func TestDispatchWrapsPortError(t *testing.T) {
	t.Parallel()
	_, err := app.New(&stubPort{err: errors.New("boom")}).Dispatch(context.Background(), "o", "r", "ci.yaml", app.DispatchInput{Ref: "main"})
	if err == nil || err.Error() != "failed to dispatch workflow: boom" {
		t.Fatalf("got %v", err)
	}
}

func TestListJobsPagesTheFullList(t *testing.T) {
	t.Parallel()
	jobs := []model.ActionRunJob{{ID: 1}, {ID: 2}, {ID: 3}}
	svc := app.New(&stubPort{jobs: jobs})
	page, err := svc.ListJobs(context.Background(), "o", "r", 9, app.Paging{Page: 1, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if page.TotalCount != 3 || len(page.Jobs) != 2 || !page.HasNext || page.NextPage != 2 {
		t.Fatalf("first page: %+v", page)
	}
	last, _ := svc.ListJobs(context.Background(), "o", "r", 9, app.Paging{Page: 2, Limit: 2})
	if len(last.Jobs) != 1 || last.Jobs[0].ID != 3 || last.HasNext || last.NextPage != 0 {
		t.Fatalf("last page: %+v", last)
	}
	beyond, _ := svc.ListJobs(context.Background(), "o", "r", 9, app.Paging{Page: 9, Limit: 2})
	if len(beyond.Jobs) != 0 || beyond.HasNext {
		t.Fatalf("beyond the end: %+v", beyond)
	}
}

func TestJobLogRejectsOverflowBeforeReading(t *testing.T) {
	t.Parallel()
	p := &stubPort{}
	_, err := app.New(p).JobLog(context.Background(), "o", "r", app.JobLogQuery{
		JobID: 1, HasOffset: true, Offset: 1<<63 - 1, MaxBytes: 5,
	})
	if !errors.Is(err, app.ErrLogRangeOverflow) {
		t.Fatalf("got %v", err)
	}
	if p.window != (app.LogWindow{}) {
		t.Fatalf("port was called with %+v", p.window)
	}
}

func TestJobLogPagesBackwardsAndForwards(t *testing.T) {
	t.Parallel()
	// A 100-byte log; the window is bytes 10-13 (4 bytes).
	p := &stubPort{log: model.JobLog{Body: []byte("abcd"), Partial: true, ContentRange: "bytes 10-13/100", ContentLength: 4}}
	page, err := app.New(p).JobLog(context.Background(), "o", "r", app.JobLogQuery{
		JobID: 1, HasOffset: true, Offset: 10, MaxBytes: 4,
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.window != (app.LogWindow{Offset: 10, HasOffset: true, MaxBytes: 4}) {
		t.Fatalf("window %+v", p.window)
	}
	if !page.TruncatedBefore || !page.TruncatedAfter {
		t.Fatalf("truncation flags: %+v", page)
	}
	if page.PreviousOffset == nil || *page.PreviousOffset != 6 || *page.PreviousMaxBytes != 4 {
		t.Fatalf("previous: %v %v", page.PreviousOffset, page.PreviousMaxBytes)
	}
	if page.NextOffset == nil || *page.NextOffset != 14 {
		t.Fatalf("next: %v", page.NextOffset)
	}
}

func TestJobLogRejectsInconsistentContentRange(t *testing.T) {
	t.Parallel()
	p := &stubPort{log: model.JobLog{Body: []byte("short"), Partial: true, ContentRange: "bytes 0-99/100"}}
	_, err := app.New(p).JobLog(context.Background(), "o", "r", app.JobLogQuery{JobID: 1, MaxBytes: 5})
	if err == nil || !strings.HasPrefix(err.Error(), "get action job logs: inconsistent Content-Range") {
		t.Fatalf("got %v", err)
	}
}

func TestRepoSecretsCarryNamesAndTimesOnly(t *testing.T) {
	t.Parallel()
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	p := &stubPort{secrets: []model.Secret{{Name: "A", Created: created}, {Name: "B"}}}
	page, err := app.New(p).ListRepoSecrets(context.Background(), "o", "r", app.Paging{Page: 1, Limit: 30})
	if err != nil {
		t.Fatal(err)
	}
	if page.Count != 2 || page.Secrets[0].CreatedAt != "2026-01-02T03:04:05Z" || page.Secrets[1].CreatedAt != "" {
		t.Fatalf("summaries: %+v", page.Secrets)
	}
}

func TestSetSecretWrapsPortError(t *testing.T) {
	t.Parallel()
	p := &stubPort{err: errors.New("denied")}
	err := app.New(p).SetOrgSecret(context.Background(), "acme", "TOKEN", "v")
	if err == nil || err.Error() != "create or update org action secret: denied" {
		t.Fatalf("got %v", err)
	}
}

// envelopeCase is one call shape over a list of n rows. Each case has a full
// page with rows after it, a short last page, an exact multiple whose last page
// is full, or a full page before an exact end.
type envelopeCase struct {
	name        string
	n           int
	page, limit int
	count       int
	hasNext     bool
}

var envelopeCases = []envelopeCase{
	{"full page with more", 5, 1, 2, 2, true},
	{"short last page", 5, 3, 2, 1, false},
	{"exact multiple last page", 4, 2, 2, 2, false},
	{"full page before exact end", 4, 1, 2, 2, true},
}

func checkEnvelope(t *testing.T, tc envelopeCase, count int, hasNext bool, page, limit int) {
	t.Helper()
	if count != tc.count || hasNext != tc.hasNext || page != tc.page || limit != tc.limit {
		t.Fatalf("count %d has_next %v page %d limit %d, want count %d has_next %v page %d limit %d",
			count, hasNext, page, limit, tc.count, tc.hasNext, tc.page, tc.limit)
	}
}

func TestRunsEnvelopeWithReportedTotal(t *testing.T) {
	t.Parallel()
	for _, tc := range envelopeCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runs := make([]model.WorkflowRun, tc.n)
			p := &stubPort{runs: runs, runTotal: int64(tc.n)}
			page, err := app.New(p).ListRuns(context.Background(), "o", "r", app.RunQuery{Paging: app.Paging{Page: tc.page, Limit: tc.limit}})
			if err != nil {
				t.Fatal(err)
			}
			checkEnvelope(t, tc, page.Count, page.HasNext, page.Page, page.Limit)
			if len(page.WorkflowRuns) != tc.count {
				t.Fatalf("rows %d, want %d", len(page.WorkflowRuns), tc.count)
			}
		})
	}
}

func TestRunsEnvelopeWithoutReportedTotalProbes(t *testing.T) {
	t.Parallel()
	for _, tc := range envelopeCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// No X-Total-Count: the rows are there but no total is reported.
			p := &stubPort{runs: make([]model.WorkflowRun, tc.n)}
			page, err := app.New(p).ListRuns(context.Background(), "o", "r", app.RunQuery{Paging: app.Paging{Page: tc.page, Limit: tc.limit}})
			if err != nil {
				t.Fatal(err)
			}
			checkEnvelope(t, tc, page.Count, page.HasNext, page.Page, page.Limit)
		})
	}
}

func TestJobsEnvelopeExactMultiple(t *testing.T) {
	t.Parallel()
	p := &stubPort{jobs: make([]model.ActionRunJob, 4)}
	svc := app.New(p)
	first, err := svc.ListJobs(context.Background(), "o", "r", 9, app.Paging{Page: 1, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if first.Count != 2 || !first.HasNext || first.TotalCount != 4 {
		t.Fatalf("first page: %+v", first)
	}
	last, err := svc.ListJobs(context.Background(), "o", "r", 9, app.Paging{Page: 2, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if last.Count != 2 || last.HasNext || last.NextPage != 0 {
		t.Fatalf("exact-multiple last page: %+v", last)
	}
}

func TestArtifactsEnvelopeWithReportedTotal(t *testing.T) {
	t.Parallel()
	for _, tc := range envelopeCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			total := tc.n
			p := &stubPort{artifacts: make([]model.ActionArtifact, tc.n), artifactTotal: &total}
			page, err := app.New(p).ListArtifacts(context.Background(), "o", "r", 9, app.ArtifactQuery{Paging: app.Paging{Page: tc.page, Limit: tc.limit}})
			if err != nil {
				t.Fatal(err)
			}
			checkEnvelope(t, tc, page.Count, page.HasNext, page.Page, page.Limit)
		})
	}
}

func TestArtifactsEnvelopeWithoutReportedTotalProbes(t *testing.T) {
	t.Parallel()
	for _, tc := range envelopeCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := &stubPort{artifacts: make([]model.ActionArtifact, tc.n)}
			page, err := app.New(p).ListArtifacts(context.Background(), "o", "r", 9, app.ArtifactQuery{Paging: app.Paging{Page: tc.page, Limit: tc.limit}})
			if err != nil {
				t.Fatal(err)
			}
			checkEnvelope(t, tc, page.Count, page.HasNext, page.Page, page.Limit)
		})
	}
}

func TestSecretsEnvelopeProbesForNext(t *testing.T) {
	t.Parallel()
	for _, tc := range envelopeCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := &stubPort{secrets: make([]model.Secret, tc.n)}
			repo, err := app.New(p).ListRepoSecrets(context.Background(), "o", "r", app.Paging{Page: tc.page, Limit: tc.limit})
			if err != nil {
				t.Fatal(err)
			}
			checkEnvelope(t, tc, repo.Count, repo.HasNext, repo.Page, repo.Limit)
			org, err := app.New(p).ListOrgSecrets(context.Background(), "acme", app.Paging{Page: tc.page, Limit: tc.limit})
			if err != nil {
				t.Fatal(err)
			}
			checkEnvelope(t, tc, org.Count, org.HasNext, org.Page, org.Limit)
		})
	}
}

// MaxResponseItems reports an unknown ceiling for the test double.
func (stubPort) MaxResponseItems(context.Context) (int, bool) { return 0, false }
