// SPDX-License-Identifier: GPL-3.0-or-later

package model

import "time"

// Actions result types. ActionRunJob and ActionArtifact mirror the wire shape
// the actions tools have always rendered (the same JSON names and order). The
// page types are the tool outputs themselves, so the use cases return the
// exact JSON a tool renders.

// WorkflowRun is the subset of a workflow run the actions tools read.
type WorkflowRun struct {
	ID          int64
	Title       string
	Status      string
	Event       string
	PrettyRef   string
	CommitSHA   string
	HTMLURL     string
	TriggerUser *string
	Created     time.Time
	Started     time.Time
	Stopped     time.Time
}

// WorkflowRunList is one page of workflow runs and the total Forgejo reports.
type WorkflowRunList struct {
	TotalCount   int64
	WorkflowRuns []WorkflowRun
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

// ArtifactList is one page of artifacts; TotalCount is nil when Forgejo did
// not report one.
type ArtifactList struct {
	Artifacts  []ActionArtifact
	TotalCount *int
}

// JobLog is a raw bounded read of a job log, as Forgejo returned it. Partial
// says Forgejo answered with only the requested range.
type JobLog struct {
	Body          []byte
	ContentType   string
	ContentRange  string
	ContentLength int64
	Partial       bool
}

// Secret is an action secret's metadata. Forgejo never returns a value.
type Secret struct {
	Name    string
	Created time.Time
}

// JobPage is the list_action_run_jobs output.
type JobPage struct {
	RunID      int64          `json:"run_id"`
	Jobs       []ActionRunJob `json:"jobs"`
	Page       int            `json:"page"`
	Limit      int            `json:"limit"`
	TotalCount int            `json:"total_count"`
	HasNext    bool           `json:"has_next"`
	NextPage   int            `json:"next_page,omitempty"`
}

// JobLogPage is the get_action_job_logs output: one window of a job log and
// how to page to its neighbours.
type JobLogPage struct {
	JobID            int64  `json:"job_id"`
	Attempt          int64  `json:"attempt,omitempty"`
	Content          string `json:"content"`
	ContentType      string `json:"content_type"`
	ContentRange     string `json:"content_range"`
	StartByte        int64  `json:"start_byte"`
	EndByte          int64  `json:"end_byte"`
	TotalBytes       int64  `json:"total_bytes"`
	BytesReturned    int    `json:"bytes_returned"`
	TruncatedBefore  bool   `json:"truncated_before"`
	TruncatedAfter   bool   `json:"truncated_after"`
	PreviousOffset   *int64 `json:"previous_offset,omitempty"`
	PreviousMaxBytes *int   `json:"previous_max_bytes,omitempty"`
	NextOffset       *int64 `json:"next_offset,omitempty"`
}

// ArtifactPage is the list_action_run_artifacts output.
type ArtifactPage struct {
	RunID      int64            `json:"run_id"`
	Artifacts  []ActionArtifact `json:"artifacts"`
	Page       int              `json:"page"`
	Limit      int              `json:"limit"`
	Count      int              `json:"count"`
	TotalCount *int             `json:"total_count,omitempty"`
}

// SecretSummary is one secret as the list tools render it. It has no value.
type SecretSummary struct {
	Name      string `json:"name"`
	CreatedAt string `json:"created_at,omitempty"`
}

// RepoSecretPage is the list_repo_action_secrets output.
type RepoSecretPage struct {
	Owner   string          `json:"owner"`
	Repo    string          `json:"repo"`
	Secrets []SecretSummary `json:"secrets"`
	Page    int             `json:"page"`
	Limit   int             `json:"limit"`
	Count   int             `json:"count"`
}

// OrgSecretPage is the list_org_action_secrets output.
type OrgSecretPage struct {
	Org     string          `json:"org"`
	Secrets []SecretSummary `json:"secrets"`
	Page    int             `json:"page"`
	Limit   int             `json:"limit"`
	Count   int             `json:"count"`
}
