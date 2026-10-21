// SPDX-License-Identifier: GPL-3.0-or-later

// Package actionsadapter implements the Actions use cases' port (internal/app/actions.Port)
// over the Forgejo port (forge.Actions). It is the one place that translates
// between repo-owned types and SDK types for the Actions domain: inputs become
// SDK options (or, for the raw-HTTP log read, an HTTP Range value), and SDK
// results become internal/app/model values.
package actionsadapter

import (
	"context"
	"fmt"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/actions"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// New returns the use cases' port over p.
// Source is the part of the Forgejo port the actions domain uses: its actions
// methods and the instance's pagination ceiling.
type Source interface {
	forge.Actions
	forge.Ceilings
}

func New(p Source) actions.Port { return adapter{p: p} }

// MaxResponseItems reads the instance's pagination ceiling through the source.
func (a adapter) MaxResponseItems(ctx context.Context) (int, bool) {
	return a.p.MaxResponseItems(ctx)
}

type adapter struct{ p Source }

var _ actions.Port = adapter{}

func (a adapter) ActionsURL(owner, repo string) string { return a.p.ActionsURL(owner, repo) }

func (a adapter) DispatchWorkflow(ctx context.Context, owner, repo, workflow string, in actions.DispatchInput) error {
	return a.p.DispatchWorkflow(ctx, owner, repo, workflow, forgejo_sdk.DispatchWorkflowOption{Ref: in.Ref, Inputs: in.Inputs})
}

func (a adapter) ListWorkflowRuns(ctx context.Context, owner, repo string, q actions.RunQuery) (model.WorkflowRunList, error) {
	resp, err := a.p.ListWorkflowRuns(ctx, owner, repo, forgejo_sdk.ListActionRunsOption{
		ListOptions: forgejo_sdk.ListOptions{Page: q.Page, PageSize: q.Limit},
		Status:      q.Status,
		Event:       q.Event,
		RunNumber:   q.RunNumber,
		HeadSHA:     q.HeadSHA,
	})
	if err != nil {
		return model.WorkflowRunList{}, err
	}
	out := model.WorkflowRunList{}
	if resp == nil {
		return out, nil
	}
	out.TotalCount = resp.TotalCount
	for _, r := range resp.WorkflowRuns {
		if r != nil {
			out.WorkflowRuns = append(out.WorkflowRuns, toRun(r))
		}
	}
	return out, nil
}

func (a adapter) GetWorkflowRun(ctx context.Context, owner, repo string, runID int64) (model.WorkflowRun, error) {
	r, err := a.p.GetWorkflowRun(ctx, owner, repo, runID)
	if err != nil {
		return model.WorkflowRun{}, err
	}
	return toRun(r), nil
}

func (a adapter) CancelWorkflowRun(ctx context.Context, owner, repo string, runID int64) error {
	return a.p.CancelWorkflowRun(ctx, owner, repo, runID)
}

func (a adapter) DeleteWorkflowRun(ctx context.Context, owner, repo string, runID int64) error {
	return a.p.DeleteWorkflowRun(ctx, owner, repo, runID)
}

func (a adapter) ListRunJobs(ctx context.Context, owner, repo string, runID int64) ([]model.ActionRunJob, error) {
	jobs, err := a.p.ListRunJobs(ctx, owner, repo, runID)
	if err != nil {
		return nil, err
	}
	var out []model.ActionRunJob
	for _, j := range jobs {
		out = append(out, model.ActionRunJob{
			ID: j.ID, RunID: j.RunID, Attempt: j.Attempt, Handle: j.Handle, RepoID: j.RepoID,
			OwnerID: j.OwnerID, Name: j.Name, Needs: j.Needs, RunsOn: j.RunsOn, TaskID: j.TaskID, Status: j.Status,
		})
	}
	return out, nil
}

// byteRange renders a window as the HTTP Range value Forgejo takes: the tail
// when there is no offset, otherwise the bytes from the offset.
func byteRange(w actions.LogWindow) string {
	if !w.HasOffset {
		return fmt.Sprintf("bytes=-%d", w.MaxBytes)
	}
	return fmt.Sprintf("bytes=%d-%d", w.Offset, w.Offset+w.MaxBytes-1)
}

func (a adapter) GetJobLog(ctx context.Context, owner, repo string, jobID, attempt int64, w actions.LogWindow) (model.JobLog, error) {
	resp, err := a.p.GetJobLog(ctx, owner, repo, jobID, attempt, byteRange(w), w.MaxBytes)
	if err != nil {
		return model.JobLog{}, err
	}
	if resp == nil {
		return model.JobLog{}, nil
	}
	return model.JobLog{
		Body: resp.Body, ContentType: resp.ContentType, ContentRange: resp.ContentRange,
		ContentLength: resp.ContentLength, Partial: resp.Partial,
	}, nil
}

func (a adapter) ListRunArtifacts(ctx context.Context, owner, repo string, runID int64, q actions.ArtifactQuery) (model.ArtifactList, error) {
	list, err := a.p.ListRunArtifacts(ctx, owner, repo, runID, forge.ArtifactListOptions{Page: q.Page, Limit: q.Limit, Name: q.Name})
	if err != nil {
		return model.ArtifactList{}, err
	}
	out := model.ArtifactList{TotalCount: list.TotalCount}
	if list.Artifacts != nil {
		out.Artifacts = make([]model.ActionArtifact, 0, len(list.Artifacts))
		for _, art := range list.Artifacts {
			out.Artifacts = append(out.Artifacts, toArtifact(art))
		}
	}
	return out, nil
}

func (a adapter) GetArtifact(ctx context.Context, owner, repo string, artifactID int64) (model.ActionArtifact, error) {
	art, err := a.p.GetArtifact(ctx, owner, repo, artifactID)
	if err != nil {
		return model.ActionArtifact{}, err
	}
	if art == nil {
		return model.ActionArtifact{}, nil
	}
	return toArtifact(*art), nil
}

func (a adapter) ListRepoSecrets(ctx context.Context, owner, repo string, p actions.Paging) ([]model.Secret, error) {
	secrets, err := a.p.ListRepoActionSecrets(ctx, owner, repo, forgejo_sdk.ListOptions{Page: p.Page, PageSize: p.Limit})
	return toSecrets(secrets), err
}

func (a adapter) SetRepoSecret(ctx context.Context, owner, repo, name, data string) error {
	return a.p.SetRepoActionSecret(ctx, owner, repo, forgejo_sdk.CreateSecretOption{Name: name, Data: data})
}

func (a adapter) DeleteRepoSecret(ctx context.Context, owner, repo, name string) error {
	return a.p.DeleteRepoActionSecret(ctx, owner, repo, name)
}

func (a adapter) ListOrgSecrets(ctx context.Context, org string, p actions.Paging) ([]model.Secret, error) {
	secrets, err := a.p.ListOrgActionSecrets(ctx, org, forgejo_sdk.ListOptions{Page: p.Page, PageSize: p.Limit})
	return toSecrets(secrets), err
}

func (a adapter) SetOrgSecret(ctx context.Context, org, name, data string) error {
	return a.p.SetOrgActionSecret(ctx, org, forgejo_sdk.CreateSecretOption{Name: name, Data: data})
}

func (a adapter) DeleteOrgSecret(ctx context.Context, org, name string) error {
	return a.p.DeleteOrgActionSecret(ctx, org, name)
}

func toRun(r *forgejo_sdk.ActionRun) model.WorkflowRun {
	run := model.WorkflowRun{
		ID: r.ID, Title: r.Title, Status: r.Status, Event: r.Event, PrettyRef: r.PrettyRef,
		CommitSHA: r.CommitSHA, HTMLURL: r.HTMLURL, Created: r.Created, Started: r.Started, Stopped: r.Stopped,
	}
	if r.TriggerUser != nil {
		name := r.TriggerUser.UserName
		run.TriggerUser = &name
	}
	return run
}

func toArtifact(a forge.ActionArtifact) model.ActionArtifact {
	return model.ActionArtifact{
		ID: a.ID, Name: a.Name, RunID: a.RunID, SizeInBytes: a.SizeInBytes, Expired: a.Expired,
		ExpiresAt: a.ExpiresAt, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt, ArchiveDownloadURL: a.ArchiveDownloadURL,
	}
}

func toSecrets(secrets []*forgejo_sdk.Secret) []model.Secret {
	out := make([]model.Secret, 0, len(secrets))
	for _, s := range secrets {
		out = append(out, model.Secret{Name: s.Name, Created: s.Created})
	}
	return out
}
