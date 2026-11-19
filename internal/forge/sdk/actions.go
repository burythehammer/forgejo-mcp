// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var _ forge.Actions = (*Adapter)(nil)

func (a *Adapter) ActionsURL(owner, repo string) string {
	return strings.TrimRight(a.raw.base, "/") + "/" + owner + "/" + repo + "/actions"
}

// DispatchWorkflow posts the dispatch raw: the SDK's no-run-info path discards
// the answer's body, which is where Forgejo says why it refused.
func (a *Adapter) DispatchWorkflow(ctx context.Context, owner, repo, workflow string, opt forgejo_sdk.DispatchWorkflowOption) error {
	path, err := forge.APIPath("repos", owner, repo, "actions", "workflows", workflow, "dispatches")
	if err != nil {
		return err
	}
	status, body, err := a.raw.doStatus(ctx, http.MethodPost, path, opt)
	if err != nil {
		return err
	}
	return answered(status, body, http.StatusOK, http.StatusNoContent)
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
	path, err := forge.APIPath("repos", owner, repo, "actions", "runs", runID, "cancel")
	if err != nil {
		return err
	}
	return a.rawDo(ctx, http.MethodPost, path, nil, nil)
}

func (a *Adapter) DeleteWorkflowRun(ctx context.Context, owner, repo string, runID int64) error {
	path, err := forge.APIPath("repos", owner, repo, "actions", "runs", runID)
	if err != nil {
		return err
	}
	return a.rawDo(ctx, http.MethodDelete, path, nil, nil)
}

func (a *Adapter) ListRunJobs(ctx context.Context, owner, repo string, runID int64) ([]forge.ActionRunJob, error) {
	path, err := forge.APIPath("repos", owner, repo, "actions", "runs", runID, "jobs")
	if err != nil {
		return nil, err
	}
	jobs := make([]forge.ActionRunJob, 0)
	if err := a.rawDo(ctx, http.MethodGet, path, nil, &jobs); err != nil {
		return nil, err
	}
	if jobs == nil {
		jobs = []forge.ActionRunJob{}
	}
	return jobs, nil
}

func (a *Adapter) GetJobLog(ctx context.Context, owner, repo string, jobID, attempt int64, byteRange string, maxBytes int64) (*forge.JobLog, error) {
	path, err := forge.APIPath("repos", owner, repo, "actions", "jobs", jobID, "logs")
	if err != nil {
		return nil, err
	}
	if attempt > 0 {
		query := url.Values{}
		query.Set("attempt", strconv.FormatInt(attempt, 10))
		path += "?" + query.Encode()
	}
	raw, err := a.raw.doAPIRaw(ctx, path, "text/plain", byteRange, maxBytes)
	if err != nil {
		return nil, err
	}
	return &forge.JobLog{
		Body:          raw.Body,
		ContentType:   raw.ContentType,
		ContentRange:  raw.ContentRange,
		ContentLength: raw.ContentLength,
		StatusCode:    raw.StatusCode,
		Partial:       raw.StatusCode == http.StatusPartialContent,
	}, nil
}

func (a *Adapter) ListRunArtifacts(ctx context.Context, owner, repo string, runID int64, opt forge.ArtifactListOptions) (forge.ArtifactList, error) {
	query := url.Values{}
	query.Set("page", strconv.Itoa(opt.Page))
	query.Set("limit", strconv.Itoa(opt.Limit))
	if opt.Name != "" {
		query.Set("name", opt.Name)
	}
	base, err := forge.APIPath("repos", owner, repo, "actions", "runs", runID, "artifacts")
	if err != nil {
		return forge.ArtifactList{}, err
	}
	path := base + "?" + query.Encode()

	artifacts := make([]forge.ActionArtifact, 0)
	header, err := a.rawJSON(ctx, http.MethodGet, path, nil, &artifacts)
	if err != nil {
		return forge.ArtifactList{}, err
	}
	// A JSON null body unmarshals onto a nil slice and overwrites the make above.
	if artifacts == nil {
		artifacts = []forge.ActionArtifact{}
	}
	return forge.ArtifactList{Artifacts: artifacts, TotalCount: totalCountPtr(header)}, nil
}

func (a *Adapter) GetArtifact(ctx context.Context, owner, repo string, artifactID int64) (*forge.ActionArtifact, error) {
	path, err := forge.APIPath("repos", owner, repo, "actions", "artifacts", artifactID)
	if err != nil {
		return nil, err
	}
	var artifact forge.ActionArtifact
	if err := a.rawDo(ctx, http.MethodGet, path, nil, &artifact); err != nil {
		return nil, err
	}
	return &artifact, nil
}

func (a *Adapter) ListRepoActionSecrets(_ context.Context, owner, repo string, opt forgejo_sdk.ListOptions) ([]*forgejo_sdk.Secret, error) {
	secrets, resp, err := a.client.ListRepoActionSecret(owner, repo, forgejo_sdk.ListRepoActionSecretOption{ListOptions: opt})
	return secrets, statusErr(resp, err)
}

// SetRepoActionSecret and the other secret writes below are raw for the same
// reason as DispatchWorkflow: the SDK turns 404 and 400 into fixed words and
// every other status into a bare one, dropping Forgejo's message.
func (a *Adapter) SetRepoActionSecret(ctx context.Context, owner, repo string, opt forgejo_sdk.CreateSecretOption) error {
	if err := opt.Validate(); err != nil {
		return err
	}
	path, err := forge.APIPath("repos", owner, repo, "actions", "secrets", opt.Name)
	if err != nil {
		return err
	}
	status, body, err := a.raw.doStatus(ctx, http.MethodPut, path, opt)
	if err != nil {
		return err
	}
	return answered(status, body, http.StatusCreated, http.StatusNoContent)
}

func (a *Adapter) DeleteRepoActionSecret(ctx context.Context, owner, repo, name string) error {
	path, err := forge.APIPath("repos", owner, repo, "actions", "secrets", name)
	if err != nil {
		return err
	}
	status, body, err := a.raw.doStatus(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	return answered(status, body, http.StatusNoContent)
}

func (a *Adapter) ListOrgActionSecrets(_ context.Context, org string, opt forgejo_sdk.ListOptions) ([]*forgejo_sdk.Secret, error) {
	secrets, resp, err := a.client.ListOrgActionSecret(org, forgejo_sdk.ListOrgActionSecretOption{ListOptions: opt})
	return secrets, statusErr(resp, err)
}

func (a *Adapter) SetOrgActionSecret(ctx context.Context, org string, opt forgejo_sdk.CreateSecretOption) error {
	if err := opt.Validate(); err != nil {
		return err
	}
	path, err := forge.APIPath("orgs", org, "actions", "secrets", opt.Name)
	if err != nil {
		return err
	}
	status, body, err := a.raw.doStatus(ctx, http.MethodPut, path, opt)
	if err != nil {
		return err
	}
	return answered(status, body, http.StatusCreated, http.StatusNoContent)
}

func (a *Adapter) DeleteOrgActionSecret(ctx context.Context, org, name string) error {
	path, err := forge.APIPath("orgs", org, "actions", "secrets", name)
	if err != nil {
		return err
	}
	status, body, err := a.raw.doStatus(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	return answered(status, body, http.StatusNoContent)
}
