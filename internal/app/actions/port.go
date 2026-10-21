// SPDX-License-Identifier: GPL-3.0-or-later

// Package actions holds the Actions use cases behind the actions tools: workflow
// dispatch and runs, jobs and logs, artifacts, and action secrets. Like all of
// internal/app/ it is MCP-free and SDK-free (the depguard rules in .golangci.yml): use
// cases take typed inputs, reach Forgejo only through Port, and return
// repo-owned types from internal/app/model for an MCP adapter in internal/operation/actions to
// render.
package actions

import (
	"context"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
)

// Port is the Forgejo Actions domain as the use cases need it, in repo-owned
// types. internal/forge/actionsadapter implements it over forge.Actions. Failures
// from Forgejo come back as ordinary errors for the use case to wrap.
type Port interface {
	// MaxResponseItems is the instance's pagination ceiling; ok is false when it
	// cannot be determined, which is "unknown", never a ceiling of zero.
	MaxResponseItems(ctx context.Context) (limit int, ok bool)
	ActionsURL(owner, repo string) string

	DispatchWorkflow(ctx context.Context, owner, repo, workflow string, in DispatchInput) error
	ListWorkflowRuns(ctx context.Context, owner, repo string, q RunQuery) (model.WorkflowRunList, error)
	GetWorkflowRun(ctx context.Context, owner, repo string, runID int64) (model.WorkflowRun, error)
	CancelWorkflowRun(ctx context.Context, owner, repo string, runID int64) error
	DeleteWorkflowRun(ctx context.Context, owner, repo string, runID int64) error

	ListRunJobs(ctx context.Context, owner, repo string, runID int64) ([]model.ActionRunJob, error)
	GetJobLog(ctx context.Context, owner, repo string, jobID, attempt int64, w LogWindow) (model.JobLog, error)

	ListRunArtifacts(ctx context.Context, owner, repo string, runID int64, q ArtifactQuery) (model.ArtifactList, error)
	GetArtifact(ctx context.Context, owner, repo string, artifactID int64) (model.ActionArtifact, error)

	ListRepoSecrets(ctx context.Context, owner, repo string, p Paging) ([]model.Secret, error)
	SetRepoSecret(ctx context.Context, owner, repo, name, data string) error
	DeleteRepoSecret(ctx context.Context, owner, repo, name string) error
	ListOrgSecrets(ctx context.Context, org string, p Paging) ([]model.Secret, error)
	SetOrgSecret(ctx context.Context, org, name, data string) error
	DeleteOrgSecret(ctx context.Context, org, name string) error
}

// DispatchInput is what triggering a workflow takes. Inputs may be nil.
type DispatchInput struct {
	Ref    string
	Inputs map[string]string
}

// RunQuery filters and pages a workflow-run list. Page and Limit are sent as
// given.
type RunQuery struct {
	Status    string
	Event     string
	RunNumber int64
	HeadSHA   string
	Paging
}

// Paging selects a page of a list.
type Paging struct{ Page, Limit int }

// ArtifactQuery pages and filters an artifact list.
type ArtifactQuery struct {
	Paging
	Name string
}

// LogWindow selects the bytes of a job log to read. With HasOffset false it is
// the tail of at most MaxBytes; otherwise MaxBytes bytes from Offset.
type LogWindow struct {
	Offset    int64
	HasOffset bool
	MaxBytes  int64
}
