// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
)

// ErrLogRangeOverflow is returned when offset and max_bytes together exceed
// the integer range.
var ErrLogRangeOverflow = errors.New("offset and max_bytes exceed the supported integer range")

// Service is the Actions use cases: one method per tool.
type Service struct{ p Port }

// New returns the use cases over p.
func New(p Port) *Service { return &Service{p: p} }

// Dispatch triggers a workflow and returns the repository's Actions page URL.
func (s *Service) Dispatch(ctx context.Context, owner, repo, workflow string, in DispatchInput) (string, error) {
	if err := s.p.DispatchWorkflow(ctx, owner, repo, workflow, in); err != nil {
		return "", fmt.Errorf("failed to dispatch workflow: %w", err)
	}
	return s.p.ActionsURL(owner, repo), nil
}

// ListRuns returns one page of workflow runs.
func (s *Service) ListRuns(ctx context.Context, owner, repo string, q RunQuery) (model.WorkflowRunList, error) {
	list, err := s.p.ListWorkflowRuns(ctx, owner, repo, q)
	if err != nil {
		return model.WorkflowRunList{}, fmt.Errorf("failed to list workflow runs: %w", err)
	}
	return list, nil
}

// GetRun returns one workflow run.
func (s *Service) GetRun(ctx context.Context, owner, repo string, runID int64) (model.WorkflowRun, error) {
	run, err := s.p.GetWorkflowRun(ctx, owner, repo, runID)
	if err != nil {
		return model.WorkflowRun{}, fmt.Errorf("failed to get workflow run: %w", err)
	}
	return run, nil
}

// CancelRun cancels a pending or running workflow run.
func (s *Service) CancelRun(ctx context.Context, owner, repo string, runID int64) error {
	if err := s.p.CancelWorkflowRun(ctx, owner, repo, runID); err != nil {
		return fmt.Errorf("cancel workflow run: %w", err)
	}
	return nil
}

// DeleteRun deletes a completed workflow run.
func (s *Service) DeleteRun(ctx context.Context, owner, repo string, runID int64) error {
	if err := s.p.DeleteWorkflowRun(ctx, owner, repo, runID); err != nil {
		return fmt.Errorf("delete workflow run: %w", err)
	}
	return nil
}

// ListJobs returns one page of a run's jobs. Forgejo does not page jobs, so
// the page is cut here.
func (s *Service) ListJobs(ctx context.Context, owner, repo string, runID int64, p Paging) (model.JobPage, error) {
	all, err := s.p.ListRunJobs(ctx, owner, repo, runID)
	if err != nil {
		return model.JobPage{}, fmt.Errorf("list action run jobs: %w", err)
	}
	start64 := int64(p.Page-1) * int64(p.Limit)
	start := len(all)
	if start64 < int64(len(all)) {
		start = int(start64)
	}
	end := min(start+p.Limit, len(all))
	result := model.JobPage{
		RunID:      runID,
		Jobs:       all[start:end],
		Page:       p.Page,
		Limit:      p.Limit,
		TotalCount: len(all),
		HasNext:    end < len(all),
	}
	if result.HasNext {
		result.NextPage = p.Page + 1
	}
	return result, nil
}

// JobLogQuery selects a window of one job's log. Offset is used only when
// HasOffset is set; otherwise the tail is read.
type JobLogQuery struct {
	JobID     int64
	Attempt   int64
	Offset    int64
	HasOffset bool
	MaxBytes  int64
}

// JobLog reads one window of a job's log and works out how to page to its
// neighbours.
func (s *Service) JobLog(ctx context.Context, owner, repo string, q JobLogQuery) (model.JobLogPage, error) {
	if q.HasOffset && q.Offset > math.MaxInt64-q.MaxBytes {
		return model.JobLogPage{}, ErrLogRangeOverflow
	}
	w := LogWindow{Offset: q.Offset, HasOffset: q.HasOffset, MaxBytes: q.MaxBytes}
	resp, err := s.p.GetJobLog(ctx, owner, repo, q.JobID, q.Attempt, w)
	if err != nil {
		return model.JobLogPage{}, fmt.Errorf("get action job logs: %w", err)
	}
	startByte, endByte, totalBytes, err := responseByteRange(resp)
	if err != nil {
		return model.JobLogPage{}, fmt.Errorf("get action job logs: %w", err)
	}
	result := model.JobLogPage{
		JobID:           q.JobID,
		Attempt:         q.Attempt,
		Content:         strings.ToValidUTF8(string(resp.Body), "�"),
		ContentType:     resp.ContentType,
		ContentRange:    resp.ContentRange,
		StartByte:       startByte,
		EndByte:         endByte,
		TotalBytes:      totalBytes,
		BytesReturned:   len(resp.Body),
		TruncatedBefore: startByte > 0,
		TruncatedAfter:  totalBytes > 0 && endByte+1 < totalBytes,
	}
	if result.TruncatedBefore {
		previousBytes := min(q.MaxBytes, startByte)
		previous := startByte - previousBytes
		previousMaxBytes := int(previousBytes)
		result.PreviousOffset = &previous
		result.PreviousMaxBytes = &previousMaxBytes
	}
	if result.TruncatedAfter {
		next := endByte + 1
		result.NextOffset = &next
	}
	return result, nil
}

// ListArtifacts returns one page of a run's artifacts.
func (s *Service) ListArtifacts(ctx context.Context, owner, repo string, runID int64, q ArtifactQuery) (model.ArtifactPage, error) {
	list, err := s.p.ListRunArtifacts(ctx, owner, repo, runID, q)
	if err != nil {
		return model.ArtifactPage{}, fmt.Errorf("list action run artifacts: %w", err)
	}
	return model.ArtifactPage{
		RunID:      runID,
		Artifacts:  list.Artifacts,
		Page:       q.Page,
		Limit:      q.Limit,
		Count:      len(list.Artifacts),
		TotalCount: list.TotalCount,
	}, nil
}

// GetArtifact returns the metadata of one artifact.
func (s *Service) GetArtifact(ctx context.Context, owner, repo string, artifactID int64) (model.ActionArtifact, error) {
	a, err := s.p.GetArtifact(ctx, owner, repo, artifactID)
	if err != nil {
		return model.ActionArtifact{}, fmt.Errorf("get action artifact: %w", err)
	}
	return a, nil
}

// ListRepoSecrets returns one page of a repository's secret names.
func (s *Service) ListRepoSecrets(ctx context.Context, owner, repo string, p Paging) (model.RepoSecretPage, error) {
	secrets, err := s.p.ListRepoSecrets(ctx, owner, repo, p)
	if err != nil {
		return model.RepoSecretPage{}, fmt.Errorf("list repo action secrets: %w", err)
	}
	summaries := toSummaries(secrets)
	return model.RepoSecretPage{
		Owner:   owner,
		Repo:    repo,
		Secrets: summaries,
		Page:    p.Page,
		Limit:   p.Limit,
		Count:   len(summaries),
	}, nil
}

// SetRepoSecret creates or overwrites a repository secret.
func (s *Service) SetRepoSecret(ctx context.Context, owner, repo, name, data string) error {
	if err := s.p.SetRepoSecret(ctx, owner, repo, name, data); err != nil {
		return fmt.Errorf("create or update repo action secret: %w", err)
	}
	return nil
}

// DeleteRepoSecret deletes a repository secret.
func (s *Service) DeleteRepoSecret(ctx context.Context, owner, repo, name string) error {
	if err := s.p.DeleteRepoSecret(ctx, owner, repo, name); err != nil {
		return fmt.Errorf("delete repo action secret: %w", err)
	}
	return nil
}

// ListOrgSecrets returns one page of an organization's secret names.
func (s *Service) ListOrgSecrets(ctx context.Context, org string, p Paging) (model.OrgSecretPage, error) {
	secrets, err := s.p.ListOrgSecrets(ctx, org, p)
	if err != nil {
		return model.OrgSecretPage{}, fmt.Errorf("list org action secrets: %w", err)
	}
	summaries := toSummaries(secrets)
	return model.OrgSecretPage{
		Org:     org,
		Secrets: summaries,
		Page:    p.Page,
		Limit:   p.Limit,
		Count:   len(summaries),
	}, nil
}

// SetOrgSecret creates or overwrites an organization secret.
func (s *Service) SetOrgSecret(ctx context.Context, org, name, data string) error {
	if err := s.p.SetOrgSecret(ctx, org, name, data); err != nil {
		return fmt.Errorf("create or update org action secret: %w", err)
	}
	return nil
}

// DeleteOrgSecret deletes an organization secret.
func (s *Service) DeleteOrgSecret(ctx context.Context, org, name string) error {
	if err := s.p.DeleteOrgSecret(ctx, org, name); err != nil {
		return fmt.Errorf("delete org action secret: %w", err)
	}
	return nil
}

// responseByteRange works out which bytes of the job log a response carried:
// the start and end offsets (inclusive) and the log's total size.
func responseByteRange(resp model.JobLog) (int64, int64, int64, error) {
	if resp.Partial {
		var start, end, total int64
		if _, err := fmt.Sscanf(resp.ContentRange, "bytes %d-%d/%d", &start, &end, &total); err != nil {
			return 0, 0, 0, fmt.Errorf("invalid Content-Range %q: %w", resp.ContentRange, err)
		}
		if start < 0 || end < start || total < end+1 || int64(len(resp.Body)) != end-start+1 {
			return 0, 0, 0, fmt.Errorf("inconsistent Content-Range %q for %d response bytes", resp.ContentRange, len(resp.Body))
		}
		return start, end, total, nil
	}
	total := resp.ContentLength
	if total < 0 {
		total = int64(len(resp.Body))
	}
	if total != int64(len(resp.Body)) {
		return 0, 0, 0, fmt.Errorf("response declared %d bytes but returned %d", total, len(resp.Body))
	}
	if total == 0 {
		return 0, -1, 0, nil
	}
	return 0, total - 1, total, nil
}

// toSummaries renders secrets as names and creation times only: a secret's
// value never leaves the use case.
func toSummaries(secrets []model.Secret) []model.SecretSummary {
	summaries := make([]model.SecretSummary, 0, len(secrets))
	for _, s := range secrets {
		summary := model.SecretSummary{Name: s.Name}
		if !s.Created.IsZero() {
			summary.CreatedAt = s.Created.Format(time.RFC3339)
		}
		summaries = append(summaries, summary)
	}
	return summaries
}
