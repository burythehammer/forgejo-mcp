// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var _ forge.Actions = (*Adapter)(nil)

// rawErr gives a raw-HTTP failure the same shape as an SDK one: a
// *forge.StatusError that keeps the helper's message.
func rawErr(err error) error {
	var he *forgejo.HTTPError
	if errors.As(err, &he) {
		return &forge.StatusError{StatusCode: he.StatusCode, Err: err}
	}
	return err
}

// rawContext points the raw-HTTP helpers at this adapter's instance.
func (a *Adapter) rawContext(ctx context.Context) context.Context {
	if a.baseURL == "" {
		return ctx
	}
	return forgejo.WithBaseURL(ctx, a.baseURL)
}

func (a *Adapter) ActionsURL(owner, repo string) string {
	base := a.baseURL
	if base == "" {
		base = forgejo.GetBaseURL()
	}
	return base + "/" + owner + "/" + repo + "/actions"
}

func (a *Adapter) DispatchWorkflow(_ context.Context, owner, repo, workflow string, opt forgejo_sdk.DispatchWorkflowOption) error {
	_, resp, err := a.client.DispatchRepoWorkflow(owner, repo, workflow, opt)
	return statusErr(resp, err)
}

func (a *Adapter) ListWorkflowRuns(_ context.Context, owner, repo string, opt forgejo_sdk.ListActionRunsOption) (*forgejo_sdk.ListActionRunsResponse, error) {
	runs, resp, err := a.client.ListRepoActionRuns(owner, repo, opt)
	return runs, statusErr(resp, err)
}

func (a *Adapter) GetWorkflowRun(_ context.Context, owner, repo string, runID int64) (*forgejo_sdk.ActionRun, error) {
	run, resp, err := a.client.GetRepoActionRun(owner, repo, runID)
	return run, statusErr(resp, err)
}

func (a *Adapter) CancelWorkflowRun(ctx context.Context, owner, repo string, runID int64) error {
	ctx = a.rawContext(ctx)
	path := forgejo.APIPath("repos", owner, repo, "actions", "runs", runID, "cancel")
	return rawErr(forgejo.DoJSON(ctx, http.MethodPost, path, nil, nil))
}

func (a *Adapter) DeleteWorkflowRun(ctx context.Context, owner, repo string, runID int64) error {
	ctx = a.rawContext(ctx)
	path := forgejo.APIPath("repos", owner, repo, "actions", "runs", runID)
	return rawErr(forgejo.DoJSON(ctx, http.MethodDelete, path, nil, nil))
}

func (a *Adapter) ListRunJobs(ctx context.Context, owner, repo string, runID int64) ([]forge.ActionRunJob, error) {
	ctx = a.rawContext(ctx)
	path := forgejo.APIPath("repos", owner, repo, "actions", "runs", runID, "jobs")
	jobs := make([]forge.ActionRunJob, 0)
	if err := forgejo.DoJSON(ctx, http.MethodGet, path, nil, &jobs); err != nil {
		return nil, rawErr(err)
	}
	if jobs == nil {
		jobs = []forge.ActionRunJob{}
	}
	return jobs, nil
}

func (a *Adapter) GetJobLog(ctx context.Context, owner, repo string, jobID, attempt int64, byteRange string, maxBytes int64) (*forge.JobLog, error) {
	ctx = a.rawContext(ctx)
	path := forgejo.APIPath("repos", owner, repo, "actions", "jobs", jobID, "logs")
	if attempt > 0 {
		query := url.Values{}
		query.Set("attempt", strconv.FormatInt(attempt, 10))
		path += "?" + query.Encode()
	}
	raw, err := forgejo.DoAPIRaw(ctx, path, "text/plain", byteRange, maxBytes)
	if err != nil {
		return nil, rawErr(err)
	}
	return &forge.JobLog{
		Body:          raw.Body,
		ContentType:   raw.ContentType,
		ContentRange:  raw.ContentRange,
		ContentLength: raw.ContentLength,
		StatusCode:    raw.StatusCode,
	}, nil
}

func (a *Adapter) ListRunArtifacts(ctx context.Context, owner, repo string, runID int64, opt forge.ArtifactListOptions) (forge.ArtifactList, error) {
	ctx = a.rawContext(ctx)
	query := url.Values{}
	query.Set("page", strconv.Itoa(opt.Page))
	query.Set("limit", strconv.Itoa(opt.Limit))
	if opt.Name != "" {
		query.Set("name", opt.Name)
	}
	path := forgejo.APIPath("repos", owner, repo, "actions", "runs", runID, "artifacts") + "?" + query.Encode()

	artifacts := make([]forge.ActionArtifact, 0)
	header, err := forgejo.DoJSONWithHeader(ctx, http.MethodGet, path, nil, &artifacts)
	if err != nil {
		return forge.ArtifactList{}, rawErr(err)
	}
	// A JSON null body unmarshals onto a nil slice and overwrites the make above.
	if artifacts == nil {
		artifacts = []forge.ActionArtifact{}
	}
	return forge.ArtifactList{Artifacts: artifacts, TotalCount: forgejo.TotalCountPtr(header)}, nil
}

func (a *Adapter) GetArtifact(ctx context.Context, owner, repo string, artifactID int64) (*forge.ActionArtifact, error) {
	ctx = a.rawContext(ctx)
	path := forgejo.APIPath("repos", owner, repo, "actions", "artifacts", artifactID)
	var artifact forge.ActionArtifact
	if err := forgejo.DoJSON(ctx, http.MethodGet, path, nil, &artifact); err != nil {
		return nil, rawErr(err)
	}
	return &artifact, nil
}

func (a *Adapter) ListRepoActionSecrets(_ context.Context, owner, repo string, opt forgejo_sdk.ListOptions) ([]*forgejo_sdk.Secret, error) {
	secrets, resp, err := a.client.ListRepoActionSecret(owner, repo, forgejo_sdk.ListRepoActionSecretOption{ListOptions: opt})
	return secrets, statusErr(resp, err)
}

func (a *Adapter) SetRepoActionSecret(_ context.Context, owner, repo string, opt forgejo_sdk.CreateSecretOption) error {
	resp, err := a.client.CreateRepoActionSecret(owner, repo, opt)
	return statusErr(resp, err)
}

func (a *Adapter) DeleteRepoActionSecret(_ context.Context, owner, repo, name string) error {
	resp, err := a.client.DeleteRepoActionSecret(owner, repo, name)
	return statusErr(resp, err)
}

func (a *Adapter) ListOrgActionSecrets(_ context.Context, org string, opt forgejo_sdk.ListOptions) ([]*forgejo_sdk.Secret, error) {
	secrets, resp, err := a.client.ListOrgActionSecret(org, forgejo_sdk.ListOrgActionSecretOption{ListOptions: opt})
	return secrets, statusErr(resp, err)
}

func (a *Adapter) SetOrgActionSecret(_ context.Context, org string, opt forgejo_sdk.CreateSecretOption) error {
	resp, err := a.client.CreateOrgActionSecret(org, opt)
	return statusErr(resp, err)
}

func (a *Adapter) DeleteOrgActionSecret(_ context.Context, org, name string) error {
	resp, err := a.client.DeleteOrgActionSecret(org, name)
	return statusErr(resp, err)
}
