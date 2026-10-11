// SPDX-License-Identifier: GPL-3.0-or-later

package forge

import (
	"context"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// Actions is the Forgejo Actions domain of the port: workflow dispatch and
// runs, their jobs, logs and artifacts, and repository and organization action
// secrets.
//
// Failures from Forgejo are *StatusError. Secrets are write-only on Forgejo:
// the list methods return names and timestamps, never values.
type Actions interface {
	// ActionsURL is the web address of a repository's Actions page.
	ActionsURL(owner, repo string) string

	DispatchWorkflow(ctx context.Context, owner, repo, workflow string, opt forgejo_sdk.DispatchWorkflowOption) error
	ListWorkflowRuns(ctx context.Context, owner, repo string, opt forgejo_sdk.ListActionRunsOption) (*forgejo_sdk.ListActionRunsResponse, error)
	GetWorkflowRun(ctx context.Context, owner, repo string, runID int64) (*forgejo_sdk.ActionRun, error)
	// CancelWorkflowRun succeeds for a run that already finished, leaving it
	// unchanged, as Forgejo does.
	CancelWorkflowRun(ctx context.Context, owner, repo string, runID int64) error
	// DeleteWorkflowRun fails for a run that is still live.
	DeleteWorkflowRun(ctx context.Context, owner, repo string, runID int64) error

	// ListRunJobs returns every job of a run; Forgejo does not page them.
	ListRunJobs(ctx context.Context, owner, repo string, runID int64) ([]ActionRunJob, error)
	// GetJobLog reads at most maxBytes of a job's plaintext log. byteRange is
	// an HTTP Range value ("bytes=-N" for the tail, "bytes=a-b"); attempt 0
	// means the latest attempt. A log larger than maxBytes is an error.
	GetJobLog(ctx context.Context, owner, repo string, jobID, attempt int64, byteRange string, maxBytes int64) (*JobLog, error)

	ListRunArtifacts(ctx context.Context, owner, repo string, runID int64, opt ArtifactListOptions) (ArtifactList, error)
	GetArtifact(ctx context.Context, owner, repo string, artifactID int64) (*ActionArtifact, error)

	ListRepoActionSecrets(ctx context.Context, owner, repo string, opt forgejo_sdk.ListOptions) ([]*forgejo_sdk.Secret, error)
	// SetRepoActionSecret creates the secret or overwrites its value.
	SetRepoActionSecret(ctx context.Context, owner, repo string, opt forgejo_sdk.CreateSecretOption) error
	DeleteRepoActionSecret(ctx context.Context, owner, repo, name string) error
	ListOrgActionSecrets(ctx context.Context, org string, opt forgejo_sdk.ListOptions) ([]*forgejo_sdk.Secret, error)
	SetOrgActionSecret(ctx context.Context, org string, opt forgejo_sdk.CreateSecretOption) error
	DeleteOrgActionSecret(ctx context.Context, org, name string) error
}

// ActionRunJob is one job of a workflow run.
type ActionRunJob struct {
	ID      int64    `json:"id"`
	RunID   int64    `json:"run_id"`
	Attempt int64    `json:"attempt"`
	Handle  string   `json:"handle"`
	RepoID  int64    `json:"repo_id"`
	OwnerID int64    `json:"owner_id"`
	Name    string   `json:"name"`
	Needs   []string `json:"needs"`
	RunsOn  []string `json:"runs_on"`
	TaskID  int64    `json:"task_id"`
	Status  string   `json:"status"`
}

// ActionArtifact is the metadata of one Actions artifact.
type ActionArtifact struct {
	ID                 int64  `json:"id"`
	Name               string `json:"name"`
	RunID              int64  `json:"run_id"`
	SizeInBytes        int64  `json:"size_in_bytes"`
	Expired            bool   `json:"expired"`
	ExpiresAt          string `json:"expires_at,omitempty"`
	CreatedAt          string `json:"created_at,omitempty"`
	UpdatedAt          string `json:"updated_at,omitempty"`
	ArchiveDownloadURL string `json:"archive_download_url,omitempty"`
}

// ArtifactListOptions pages and filters ListRunArtifacts. Page and Limit are
// sent as given; Name, when set, filters by artifact name.
type ArtifactListOptions struct {
	Page  int
	Limit int
	Name  string
}

// ArtifactList is one page of artifacts. TotalCount is nil when Forgejo did
// not report one.
type ArtifactList struct {
	Artifacts  []ActionArtifact
	TotalCount *int
}

// JobLog is a bounded read of a job log. StatusCode is 206 when Forgejo
// honoured the range; ContentLength is -1 when unknown.
type JobLog struct {
	Body          []byte
	ContentType   string
	ContentRange  string
	ContentLength int64
	StatusCode    int
}
