// SPDX-License-Identifier: GPL-3.0-or-later

package fake

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var _ forge.Actions = (*Forgejo)(nil)

// FakeBaseURL is the web address the fake reports for its instance.
const FakeBaseURL = "https://forgejo.example"

type actionsState struct {
	mu        sync.Mutex
	repos     map[string]*actionsRepo // key "owner/repo"
	orgs      map[string]map[string]*forgejo_sdk.Secret
	nextRunID int64
	nextJobID int64
	nextArtID int64
}

type actionsRepo struct {
	workflows map[string]bool
	runs      []*runRecord
	secrets   map[string]*forgejo_sdk.Secret
}

type runRecord struct {
	run       *forgejo_sdk.ActionRun
	jobs      []forge.ActionRunJob
	logs      map[int64]string // by job ID
	artifacts []forge.ActionArtifact
}

// live reports whether a run can still be cancelled.
func (r *runRecord) live() bool {
	switch r.run.Status {
	case "waiting", "running", "queued", "blocked", "pending":
		return true
	}
	return false
}

func (s *actionsState) repo(owner, repo string) *actionsRepo {
	if s.repos == nil {
		s.repos = map[string]*actionsRepo{}
	}
	key := owner + "/" + repo
	r, ok := s.repos[key]
	if !ok {
		r = &actionsRepo{workflows: map[string]bool{}, secrets: map[string]*forgejo_sdk.Secret{}}
		s.repos[key] = r
	}
	return r
}

func (s *actionsState) run(owner, repo string, runID int64) (*runRecord, error) {
	for _, r := range s.repo(owner, repo).runs {
		if r.run.ID == runID {
			return r, nil
		}
	}
	return nil, notFound("workflow run")
}

// AddActionsOrg makes an organization exist for its action secrets.
func (f *Forgejo) AddActionsOrg(org string) {
	f.actions.mu.Lock()
	defer f.actions.mu.Unlock()
	if f.actions.orgs == nil {
		f.actions.orgs = map[string]map[string]*forgejo_sdk.Secret{}
	}
	if _, ok := f.actions.orgs[org]; !ok {
		f.actions.orgs[org] = map[string]*forgejo_sdk.Secret{}
	}
}

// AddWorkflow declares a workflow file that DispatchWorkflow can trigger.
func (f *Forgejo) AddWorkflow(owner, repo, name string) {
	f.actions.mu.Lock()
	defer f.actions.mu.Unlock()
	f.actions.repo(owner, repo).workflows[name] = true
}

// AddWorkflowRun seeds a run, which the port cannot create (dispatching only
// queues one). Zero ID and RunNumber are assigned; the stored copy is returned.
func (f *Forgejo) AddWorkflowRun(owner, repo string, run forgejo_sdk.ActionRun) *forgejo_sdk.ActionRun {
	f.actions.mu.Lock()
	defer f.actions.mu.Unlock()
	return f.actions.addRun(owner, repo, run)
}

func (s *actionsState) addRun(owner, repo string, run forgejo_sdk.ActionRun) *forgejo_sdk.ActionRun {
	r := s.repo(owner, repo)
	if run.ID == 0 {
		s.nextRunID++
		run.ID = s.nextRunID
	} else if run.ID > s.nextRunID {
		s.nextRunID = run.ID
	}
	if run.RunNumber == 0 {
		run.RunNumber = int64(len(r.runs) + 1)
	}
	rec := &runRecord{run: &run, logs: map[int64]string{}}
	r.runs = append(r.runs, rec)
	cp := run
	return &cp
}

// AddRunJob seeds a job of a run and returns it with its assigned ID.
func (f *Forgejo) AddRunJob(owner, repo string, runID int64, job forge.ActionRunJob) (forge.ActionRunJob, error) {
	f.actions.mu.Lock()
	defer f.actions.mu.Unlock()
	rec, err := f.actions.run(owner, repo, runID)
	if err != nil {
		return job, err
	}
	f.actions.nextJobID++
	job.ID = f.actions.nextJobID
	job.RunID = runID
	if job.Attempt == 0 {
		job.Attempt = 1
	}
	rec.jobs = append(rec.jobs, job)
	return job, nil
}

// SetJobLog sets the plaintext log of a job.
func (f *Forgejo) SetJobLog(owner, repo string, jobID int64, log string) error {
	f.actions.mu.Lock()
	defer f.actions.mu.Unlock()
	for _, rec := range f.actions.repo(owner, repo).runs {
		for _, j := range rec.jobs {
			if j.ID == jobID {
				rec.logs[jobID] = log
				return nil
			}
		}
	}
	return notFound("job")
}

// AddRunArtifact seeds an artifact of a run and returns it with its ID.
func (f *Forgejo) AddRunArtifact(owner, repo string, runID int64, a forge.ActionArtifact) (forge.ActionArtifact, error) {
	f.actions.mu.Lock()
	defer f.actions.mu.Unlock()
	rec, err := f.actions.run(owner, repo, runID)
	if err != nil {
		return a, err
	}
	f.actions.nextArtID++
	a.ID = f.actions.nextArtID
	a.RunID = runID
	rec.artifacts = append(rec.artifacts, a)
	return a, nil
}

func (f *Forgejo) ActionsURL(owner, repo string) string {
	return FakeBaseURL + "/" + owner + "/" + repo + "/actions"
}

func (f *Forgejo) DispatchWorkflow(_ context.Context, owner, repo, workflow string, opt forgejo_sdk.DispatchWorkflowOption) error {
	if !f.repoExists(owner, repo) {
		return notFound("repository")
	}
	f.actions.mu.Lock()
	defer f.actions.mu.Unlock()
	r := f.actions.repo(owner, repo)
	if !r.workflows[workflow] {
		return notFound("workflow")
	}
	if opt.Ref == "" {
		return statusErr(http.StatusUnprocessableEntity, "ref is required")
	}
	now := f.Now()
	f.actions.addRun(owner, repo, forgejo_sdk.ActionRun{
		WorkflowID:  workflow,
		Title:       workflow,
		Status:      "waiting",
		Event:       "workflow_dispatch",
		PrettyRef:   opt.Ref,
		TriggerUser: user(Poster),
		Created:     now,
		Updated:     now,
	})
	return nil
}

func (f *Forgejo) ListWorkflowRuns(_ context.Context, owner, repo string, opt forgejo_sdk.ListActionRunsOption) (*forgejo_sdk.ListActionRunsResponse, error) {
	if !f.repoExists(owner, repo) {
		return nil, notFound("repository")
	}
	f.actions.mu.Lock()
	defer f.actions.mu.Unlock()
	var out []*forgejo_sdk.ActionRun
	for _, rec := range f.actions.repo(owner, repo).runs {
		run := rec.run
		if (opt.Status != "" && run.Status != opt.Status) ||
			(opt.Event != "" && run.Event != opt.Event) ||
			(opt.RunNumber > 0 && run.RunNumber != opt.RunNumber) ||
			(opt.HeadSHA != "" && run.CommitSHA != opt.HeadSHA) {
			continue
		}
		cp := *run
		out = append(out, &cp)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	lo, hi := page(opt.ListOptions, len(out))
	return &forgejo_sdk.ListActionRunsResponse{TotalCount: int64(len(out)), WorkflowRuns: out[lo:hi]}, nil
}

func (f *Forgejo) GetWorkflowRun(_ context.Context, owner, repo string, runID int64) (*forgejo_sdk.ActionRun, error) {
	f.actions.mu.Lock()
	defer f.actions.mu.Unlock()
	rec, err := f.actions.run(owner, repo, runID)
	if err != nil {
		return nil, err
	}
	cp := *rec.run
	return &cp, nil
}

func (f *Forgejo) CancelWorkflowRun(_ context.Context, owner, repo string, runID int64) error {
	f.actions.mu.Lock()
	defer f.actions.mu.Unlock()
	rec, err := f.actions.run(owner, repo, runID)
	if err != nil {
		return err
	}
	if rec.live() {
		rec.run.Status = "cancelled"
		rec.run.Stopped = f.Now()
	}
	return nil
}

func (f *Forgejo) DeleteWorkflowRun(_ context.Context, owner, repo string, runID int64) error {
	f.actions.mu.Lock()
	defer f.actions.mu.Unlock()
	rec, err := f.actions.run(owner, repo, runID)
	if err != nil {
		return err
	}
	if rec.live() {
		return statusErr(http.StatusBadRequest, "this workflow run is not done")
	}
	r := f.actions.repo(owner, repo)
	for i, x := range r.runs {
		if x == rec {
			r.runs = append(r.runs[:i], r.runs[i+1:]...)
			break
		}
	}
	return nil
}

func (f *Forgejo) ListRunJobs(_ context.Context, owner, repo string, runID int64) ([]forge.ActionRunJob, error) {
	f.actions.mu.Lock()
	defer f.actions.mu.Unlock()
	rec, err := f.actions.run(owner, repo, runID)
	if err != nil {
		return nil, err
	}
	return append([]forge.ActionRunJob{}, rec.jobs...), nil
}

func (f *Forgejo) GetJobLog(_ context.Context, owner, repo string, jobID, attempt int64, byteRange string, maxBytes int64) (*forge.JobLog, error) {
	if maxBytes <= 0 {
		return nil, errors.New("maxBytes must be positive")
	}
	f.actions.mu.Lock()
	defer f.actions.mu.Unlock()
	for _, rec := range f.actions.repo(owner, repo).runs {
		for _, j := range rec.jobs {
			if j.ID != jobID {
				continue
			}
			if attempt > 0 && attempt != j.Attempt {
				return nil, notFound("job attempt")
			}
			return serveLog(rec.logs[jobID], byteRange, maxBytes)
		}
	}
	return nil, notFound("job")
}

// serveLog answers a ranged read of log the way Forgejo does: 206 with a
// Content-Range for a satisfiable range, 200 with the whole log otherwise.
func serveLog(log, byteRange string, maxBytes int64) (*forge.JobLog, error) {
	total := int64(len(log))
	out := &forge.JobLog{ContentType: "text/plain; charset=utf-8", StatusCode: http.StatusOK}
	body := log
	if spec, ok := strings.CutPrefix(byteRange, "bytes="); ok && total > 0 {
		start, end, err := parseRange(spec, total)
		if err != nil {
			return nil, statusErr(http.StatusRequestedRangeNotSatisfiable, "range not satisfiable")
		}
		body = log[start : end+1]
		out.StatusCode = http.StatusPartialContent
		out.ContentRange = fmt.Sprintf("bytes %d-%d/%d", start, end, total)
	}
	if int64(len(body)) > maxBytes {
		return nil, errors.New("payload exceeds inline cap")
	}
	out.Body = []byte(body)
	out.ContentLength = int64(len(body))
	return out, nil
}

// parseRange resolves "a-b", "a-" and "-n" against total.
func parseRange(spec string, total int64) (start, end int64, err error) {
	lo, hi, ok := strings.Cut(spec, "-")
	if !ok {
		return 0, 0, errors.New("bad range")
	}
	if lo == "" {
		n, perr := strconv.ParseInt(hi, 10, 64)
		if perr != nil || n <= 0 {
			return 0, 0, errors.New("bad range")
		}
		return max(total-n, 0), total - 1, nil
	}
	{
		start, err = strconv.ParseInt(lo, 10, 64)
		if err != nil || start >= total {
			return 0, 0, errors.New("bad range")
		}
		end = total - 1
		if hi != "" {
			e, perr := strconv.ParseInt(hi, 10, 64)
			if perr != nil || e < start {
				return 0, 0, errors.New("bad range")
			}
			end = min(e, total-1)
		}
		return start, end, nil
	}
}

func (f *Forgejo) ListRunArtifacts(_ context.Context, owner, repo string, runID int64, opt forge.ArtifactListOptions) (forge.ArtifactList, error) {
	f.actions.mu.Lock()
	defer f.actions.mu.Unlock()
	rec, err := f.actions.run(owner, repo, runID)
	if err != nil {
		return forge.ArtifactList{}, err
	}
	var out []forge.ActionArtifact
	for _, a := range rec.artifacts {
		if opt.Name == "" || a.Name == opt.Name {
			out = append(out, a)
		}
	}
	total := len(out)
	lo, hi := page(forgejo_sdk.ListOptions{Page: opt.Page, PageSize: opt.Limit}, total)
	pg := out[lo:hi]
	if pg == nil {
		pg = []forge.ActionArtifact{}
	}
	return forge.ArtifactList{Artifacts: pg, TotalCount: &total}, nil
}

func (f *Forgejo) GetArtifact(_ context.Context, owner, repo string, artifactID int64) (*forge.ActionArtifact, error) {
	f.actions.mu.Lock()
	defer f.actions.mu.Unlock()
	for _, rec := range f.actions.repo(owner, repo).runs {
		for _, a := range rec.artifacts {
			if a.ID == artifactID {
				cp := a
				return &cp, nil
			}
		}
	}
	return nil, notFound("artifact")
}

func listSecrets(secrets map[string]*forgejo_sdk.Secret, opt forgejo_sdk.ListOptions) []*forgejo_sdk.Secret {
	all := make([]*forgejo_sdk.Secret, 0, len(secrets))
	for _, s := range secrets {
		// Forgejo never returns a secret's value.
		all = append(all, &forgejo_sdk.Secret{Name: s.Name, Created: s.Created})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Name < all[j].Name })
	lo, hi := page(opt, len(all))
	return all[lo:hi]
}

func (f *Forgejo) putSecret(secrets map[string]*forgejo_sdk.Secret, opt forgejo_sdk.CreateSecretOption) error {
	if err := opt.Validate(); err != nil {
		return err
	}
	name := strings.ToUpper(opt.Name)
	if old, ok := secrets[name]; ok {
		old.Data = opt.Data
		return nil
	}
	secrets[name] = &forgejo_sdk.Secret{Name: name, Data: opt.Data, Created: f.Now()}
	return nil
}

func dropSecret(secrets map[string]*forgejo_sdk.Secret, name string) error {
	key := strings.ToUpper(name)
	if _, ok := secrets[key]; !ok {
		return notFound("secret")
	}
	delete(secrets, key)
	return nil
}

func (f *Forgejo) ListRepoActionSecrets(_ context.Context, owner, repo string, opt forgejo_sdk.ListOptions) ([]*forgejo_sdk.Secret, error) {
	if !f.repoExists(owner, repo) {
		return nil, notFound("repository")
	}
	f.actions.mu.Lock()
	defer f.actions.mu.Unlock()
	return listSecrets(f.actions.repo(owner, repo).secrets, opt), nil
}

func (f *Forgejo) SetRepoActionSecret(_ context.Context, owner, repo string, opt forgejo_sdk.CreateSecretOption) error {
	if !f.repoExists(owner, repo) {
		return notFound("repository")
	}
	f.actions.mu.Lock()
	defer f.actions.mu.Unlock()
	return f.putSecret(f.actions.repo(owner, repo).secrets, opt)
}

func (f *Forgejo) DeleteRepoActionSecret(_ context.Context, owner, repo, name string) error {
	if !f.repoExists(owner, repo) {
		return notFound("repository")
	}
	f.actions.mu.Lock()
	defer f.actions.mu.Unlock()
	return dropSecret(f.actions.repo(owner, repo).secrets, name)
}

func (f *Forgejo) orgSecrets(org string) (map[string]*forgejo_sdk.Secret, error) {
	s, ok := f.actions.orgs[org]
	if !ok {
		return nil, notFound("organization")
	}
	return s, nil
}

func (f *Forgejo) ListOrgActionSecrets(_ context.Context, org string, opt forgejo_sdk.ListOptions) ([]*forgejo_sdk.Secret, error) {
	f.actions.mu.Lock()
	defer f.actions.mu.Unlock()
	s, err := f.orgSecrets(org)
	if err != nil {
		return nil, err
	}
	return listSecrets(s, opt), nil
}

func (f *Forgejo) SetOrgActionSecret(_ context.Context, org string, opt forgejo_sdk.CreateSecretOption) error {
	f.actions.mu.Lock()
	defer f.actions.mu.Unlock()
	s, err := f.orgSecrets(org)
	if err != nil {
		return err
	}
	return f.putSecret(s, opt)
}

func (f *Forgejo) DeleteOrgActionSecret(_ context.Context, org, name string) error {
	f.actions.mu.Lock()
	defer f.actions.mu.Unlock()
	s, err := f.orgSecrets(org)
	if err != nil {
		return err
	}
	return dropSecret(s, name)
}
