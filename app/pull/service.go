// SPDX-License-Identifier: GPL-3.0-or-later

package pull

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/list"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/split"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/diff"
)

// Service runs the pull-request use cases over a Port. Build one per request
// from the port the request's credential selects; it holds no other state.
type Service struct{ port Port }

// New returns the use cases bound to port.
func New(port Port) *Service { return &Service{port: port} }

// PR names a pull request.
type PR struct {
	Owner, Repo string
	Index       int64
}

// PagedPR is a PR plus the page of a list under it. Zero Page and Limit mean
// page 1 and the use case's default size.
type PagedPR struct {
	PR
	Page, Limit int
}

// ReviewRef names one review of a pull request.
type ReviewRef struct {
	PR
	ID int64
}

// ListInput lists a repository's pull requests. State defaults to "open",
// Page to 1 and Limit to 20; Milestone is a milestone ID as text.
type ListInput struct {
	Owner, Repo, State, Sort, Milestone string
	Page, Limit                         int
}

// CreateRequest opens a pull request.
type CreateRequest struct{ Owner, Repo, Head, Base, Title, Body string }

// UpdateInput changes a pull request; empty fields are left alone.
type UpdateInput struct {
	PR
	Title, Body, Base, Assignee, Milestone string
}

// MergeRequest merges a pull request.
type MergeRequest struct {
	PR
	Style, Title, Message                  string
	DeleteBranch, Force, WhenChecksSucceed bool
}

// MergeResult reports a merge that was accepted: carried out now, or
// scheduled for when checks succeed.
type MergeResult struct{ Scheduled bool }

// DiffInput fetches a pull request's diff, optionally of one file.
type DiffInput struct {
	PR
	FilePath string
}

// CreateReviewRequest creates a review; Comments is the JSON array of inline
// comments the tool takes, or empty.
type CreateReviewRequest struct {
	PR
	State, Body, Comments string
}

// SubmitReviewRequest submits a pending review.
type SubmitReviewRequest struct {
	ReviewRef
	State, Body string
}

// DismissRequest dismisses a review.
type DismissRequest struct {
	ReviewRef
	Message string
}

// ReviewersInput names reviewers as comma-separated lists of users and teams.
type ReviewersInput struct {
	PR
	Reviewers, TeamReviewers string
}

const (
	defaultLimit      = 20
	defaultFilesLimit = 50
)

func paging(page, limit, defLimit int, c list.Ceiling) Paging {
	if page == 0 {
		page = 1
	}
	if limit == 0 {
		limit = list.DefaultLimit(defLimit, c)
	}
	return Paging{Page: page, Limit: limit}
}

// ceiling reads the instance's pagination ceiling through the port, per call.
func (s *Service) ceiling(ctx context.Context) list.Ceiling {
	max, known := s.port.MaxResponseItems(ctx)
	return list.Ceiling{Max: max, Known: known}
}

func milestoneID(s string) (int64, error) {
	if s == "" {
		return 0, nil
	}
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, &InvalidInputError{What: "invalid milestone ID", Err: err}
	}
	return id, nil
}

// List returns a repository's pull requests.
func (s *Service) List(ctx context.Context, in ListInput) ([]*model.PullRequest, error) {
	c := s.ceiling(ctx)
	q := ListQuery{State: in.State, Sort: in.Sort, Paging: paging(in.Page, in.Limit, defaultLimit, c)}
	if q.State == "" {
		q.State = "open"
	}
	var err error
	if q.Milestone, err = milestoneID(in.Milestone); err != nil {
		return nil, err
	}
	return list.Items(ctx, list.Paging(q.Paging), c, func(ctx context.Context, p list.Paging) (list.Batch[*model.PullRequest], error) {
		qq := q
		qq.Paging = Paging(p)
		prs, err := s.port.ListPullRequests(ctx, in.Owner, in.Repo, qq)
		if err != nil {
			return list.Batch[*model.PullRequest]{}, fmt.Errorf("get pull request list err: %w", err)
		}
		return list.Batch[*model.PullRequest]{Items: prs}, nil
	})
}

// Create opens a pull request.
func (s *Service) Create(ctx context.Context, in CreateRequest) (*model.PullRequest, error) {
	pr, err := s.port.CreatePullRequest(ctx, in.Owner, in.Repo, CreateInput{Head: in.Head, Base: in.Base, Title: in.Title, Body: in.Body})
	if err != nil {
		return nil, fmt.Errorf("create pull request err: %w", err)
	}
	return pr, nil
}

// Update changes the fields of a pull request that are set.
func (s *Service) Update(ctx context.Context, in UpdateInput) (*model.PullRequest, error) {
	edit := EditInput{Title: in.Title, Base: in.Base, Assignee: in.Assignee}
	if in.Body != "" {
		body := in.Body
		edit.Body = &body
	}
	var err error
	if edit.Milestone, err = milestoneID(in.Milestone); err != nil {
		return nil, err
	}
	pr, err := s.port.EditPullRequest(ctx, in.Owner, in.Repo, in.Index, edit)
	if err != nil {
		return nil, fmt.Errorf("update pull request err: %w", err)
	}
	return pr, nil
}

// Merge merges a pull request. A merge Forgejo declines is a
// *MergeDeclinedError, unless it was only to be scheduled.
func (s *Service) Merge(ctx context.Context, in MergeRequest) (MergeResult, error) {
	out, err := s.port.MergePullRequest(ctx, in.Owner, in.Repo, in.Index, MergeInput{
		Style: in.Style, Title: in.Title, Message: in.Message,
		DeleteBranch: in.DeleteBranch, Force: in.Force, WhenChecksSucceed: in.WhenChecksSucceed,
	})
	if err != nil {
		return MergeResult{}, fmt.Errorf("merge pull request err: %w", err)
	}
	if !out.Merged && !in.WhenChecksSucceed {
		return MergeResult{}, &MergeDeclinedError{StatusCode: out.StatusCode}
	}
	return MergeResult{Scheduled: in.WhenChecksSucceed}, nil
}

// Files lists the changed files of a pull request.
func (s *Service) Files(ctx context.Context, in PagedPR) ([]*model.ChangedFile, error) {
	c := s.ceiling(ctx)
	return list.Items(ctx, list.Paging(paging(in.Page, in.Limit, defaultFilesLimit, c)), c, func(ctx context.Context, p list.Paging) (list.Batch[*model.ChangedFile], error) {
		files, err := s.port.ListPullRequestFiles(ctx, in.Owner, in.Repo, in.Index, Paging(p))
		if err != nil {
			return list.Batch[*model.ChangedFile]{}, fmt.Errorf("list pull request files err: %w", err)
		}
		return list.Batch[*model.ChangedFile]{Items: files}, nil
	})
}

// Diff returns a pull request's unified diff, or only the section for
// in.FilePath (*DiffFileNotFoundError when the diff has none).
func (s *Service) Diff(ctx context.Context, in DiffInput) (string, error) {
	body, err := s.port.GetPullRequestDiff(ctx, in.Owner, in.Repo, in.Index)
	if err != nil {
		return "", fmt.Errorf("get pull request diff err: %w", err)
	}
	if in.FilePath == "" {
		return body, nil
	}
	slice, found := diff.FileSlice(body, in.FilePath)
	if !found {
		return "", &DiffFileNotFoundError{Path: in.FilePath}
	}
	return slice, nil
}

// Reviews lists the reviews of a pull request.
func (s *Service) Reviews(ctx context.Context, in PagedPR) ([]*model.PullReview, error) {
	c := s.ceiling(ctx)
	return list.Items(ctx, list.Paging(paging(in.Page, in.Limit, defaultLimit, c)), c, func(ctx context.Context, p list.Paging) (list.Batch[*model.PullReview], error) {
		reviews, err := s.port.ListPullReviews(ctx, in.Owner, in.Repo, in.Index, Paging(p))
		if err != nil {
			return list.Batch[*model.PullReview]{}, fmt.Errorf("list pull reviews err: %w", err)
		}
		return list.Batch[*model.PullReview]{Items: reviews}, nil
	})
}

// Review returns one review.
func (s *Service) Review(ctx context.Context, in ReviewRef) (*model.PullReview, error) {
	review, err := s.port.GetPullReview(ctx, in.Owner, in.Repo, in.Index, in.ID)
	if err != nil {
		return nil, fmt.Errorf("get pull review err: %w", err)
	}
	return review, nil
}

// ReviewComments lists the inline comments of a review.
func (s *Service) ReviewComments(ctx context.Context, in ReviewRef) ([]*model.PullReviewComment, error) {
	comments, err := s.port.ListPullReviewComments(ctx, in.Owner, in.Repo, in.Index, in.ID)
	if err != nil {
		return nil, fmt.Errorf("list pull review comments err: %w", err)
	}
	return comments, nil
}

// CreateReview creates a review, with inline comments when in.Comments is a
// JSON array of them.
func (s *Service) CreateReview(ctx context.Context, in CreateReviewRequest) (*model.PullReview, error) {
	create := CreateReviewInput{State: in.State, Body: in.Body}
	if in.Comments != "" {
		if err := json.Unmarshal([]byte(in.Comments), &create.Comments); err != nil {
			return nil, &InvalidInputError{What: "invalid comments JSON", Err: err}
		}
	}
	review, err := s.port.CreatePullReview(ctx, in.Owner, in.Repo, in.Index, create)
	if err != nil {
		return nil, fmt.Errorf("create pull review err: %w", err)
	}
	return review, nil
}

// SubmitReview submits a pending review.
func (s *Service) SubmitReview(ctx context.Context, in SubmitReviewRequest) (*model.PullReview, error) {
	review, err := s.port.SubmitPullReview(ctx, in.Owner, in.Repo, in.Index, in.ID, SubmitReviewInput{State: in.State, Body: in.Body})
	if err != nil {
		return nil, fmt.Errorf("submit pull review err: %w", err)
	}
	return review, nil
}

// DismissReview dismisses a review with a message.
func (s *Service) DismissReview(ctx context.Context, in DismissRequest) error {
	if err := s.port.DismissPullReview(ctx, in.Owner, in.Repo, in.Index, in.ID, in.Message); err != nil {
		return fmt.Errorf("dismiss pull review err: %w", err)
	}
	return nil
}

// DeleteReview deletes a pending review.
func (s *Service) DeleteReview(ctx context.Context, in ReviewRef) error {
	if err := s.port.DeletePullReview(ctx, in.Owner, in.Repo, in.Index, in.ID); err != nil {
		return fmt.Errorf("delete pull review err: %w", err)
	}
	return nil
}

// RequestReviews asks users and teams for review.
func (s *Service) RequestReviews(ctx context.Context, in ReviewersInput) error {
	if err := s.port.CreateReviewRequests(ctx, in.Owner, in.Repo, in.Index, reviewers(in)); err != nil {
		return fmt.Errorf("create review requests err: %w", err)
	}
	return nil
}

// CancelReviewRequests withdraws pending review requests.
func (s *Service) CancelReviewRequests(ctx context.Context, in ReviewersInput) error {
	if err := s.port.DeleteReviewRequests(ctx, in.Owner, in.Repo, in.Index, reviewers(in)); err != nil {
		return fmt.Errorf("delete review requests err: %w", err)
	}
	return nil
}

func reviewers(in ReviewersInput) Reviewers {
	var r Reviewers
	if in.Reviewers != "" {
		r.Users = split.CSV(in.Reviewers)
	}
	if in.TeamReviewers != "" {
		r.Teams = split.CSV(in.TeamReviewers)
	}
	return r
}
