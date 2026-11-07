// SPDX-License-Identifier: GPL-3.0-or-later

package pull

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/labelresolve"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/list"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/split"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/diff"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/ptr"
)

// Service runs the pull-request use cases over a Port. Build one per request
// from the port the request's credential selects; it holds no other state.
type Service struct {
	port   Port
	labels *labelresolve.Resolver
}

// New returns the use cases bound to port.
func New(port Port) *Service {
	return &Service{port: port, labels: labelresolve.New(catalog{port})}
}

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

// CreateRequest opens a pull request. Labels and Assignees are comma-separated
// and nil when not given; Labels take label names or numeric IDs.
type CreateRequest struct {
	Owner, Repo, Head, Base, Title, Body string
	Labels, Assignees                    *string
}

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

// DiffInput fetches a pull request's diff, optionally of one file, or one page
// of its files when FilePath is empty. Page and Limit are 1-based and count files.
type DiffInput struct {
	PR
	FilePath    string
	Page, Limit int
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
	// defaultDiffLimit is the number of files in one diff page. It matches the
	// files list, so a pull request of ordinary size comes back whole.
	defaultDiffLimit = 50
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

// List returns one page of a repository's pull requests with its next marker.
func (s *Service) List(ctx context.Context, in ListInput) (list.Bounded[*model.PullRequest], error) {
	c := s.ceiling(ctx)
	q := ListQuery{State: in.State, Sort: in.Sort, Paging: paging(in.Page, in.Limit, defaultLimit, c)}
	if q.State == "" {
		q.State = "open"
	}
	var err error
	if q.Milestone, err = milestoneID(in.Milestone); err != nil {
		return list.Bounded[*model.PullRequest]{}, err
	}
	page, err := list.Run(ctx, list.Request{Page: in.Page, Limit: in.Limit, Def: defaultLimit}, c, func(ctx context.Context, p list.Paging) (list.Batch[*model.PullRequest], error) {
		qq := q
		qq.Paging = Paging(p)
		prs, err := s.port.ListPullRequests(ctx, in.Owner, in.Repo, qq)
		if err != nil {
			return list.Batch[*model.PullRequest]{}, fmt.Errorf("get pull request list err: %w", err)
		}
		return list.Batch[*model.PullRequest]{Items: prs}, nil
	})
	if err != nil {
		return list.Bounded[*model.PullRequest]{}, err
	}
	return page, nil
}

// Create opens a pull request. Label names are resolved before the port is
// called, so an unknown label fails the call and opens nothing.
func (s *Service) Create(ctx context.Context, in CreateRequest) (*model.PullRequest, error) {
	opt := CreateInput{Head: in.Head, Base: in.Base, Title: in.Title, Body: in.Body}
	if in.Labels != nil {
		ids, err := s.labels.Resolve(ctx, in.Owner, in.Repo, *in.Labels)
		if err != nil {
			return nil, err
		}
		opt.Labels = ids
	}
	if in.Assignees != nil {
		opt.Assignees = split.CSV(*in.Assignees)
	}
	pr, err := s.port.CreatePullRequest(ctx, in.Owner, in.Repo, opt)
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

// statusCreated is the HTTP status of a merge scheduled for when checks succeed.
const statusCreated = 201

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
	// 201 is Forgejo's answer to a merge scheduled for when checks succeed. Any
	// other non-merged answer is a decline, scheduled or not, and keeps its
	// status and Forgejo's message.
	scheduled := in.WhenChecksSucceed && out.StatusCode == statusCreated
	if !out.Merged && !scheduled {
		return MergeResult{}, &MergeDeclinedError{StatusCode: out.StatusCode, Message: out.Message}
	}
	return MergeResult{Scheduled: in.WhenChecksSucceed}, nil
}

// Files lists one page of the changed files of a pull request with its next marker.
func (s *Service) Files(ctx context.Context, in PagedPR) (list.Bounded[*model.ChangedFile], error) {
	c := s.ceiling(ctx)
	page, err := list.Run(ctx, list.Request{Page: in.Page, Limit: in.Limit, Def: defaultFilesLimit}, c, func(ctx context.Context, p list.Paging) (list.Batch[*model.ChangedFile], error) {
		files, err := s.port.ListPullRequestFiles(ctx, in.Owner, in.Repo, in.Index, Paging(p))
		if err != nil {
			return list.Batch[*model.ChangedFile]{}, fmt.Errorf("list pull request files err: %w", err)
		}
		return list.Batch[*model.ChangedFile]{Items: files}, nil
	})
	if err != nil {
		return list.Bounded[*model.ChangedFile]{}, err
	}
	return page, nil
}

// Diff returns a pull request's unified diff. With in.FilePath it returns only
// that file's section (*DiffFileNotFoundError when the diff has none) and no
// page. Otherwise it returns one page of whole files, in.Limit files from
// in.Page; a page of a diff that fits in one page is the whole diff.
func (s *Service) Diff(ctx context.Context, in DiffInput) (DiffPage, error) {
	body, err := s.port.GetPullRequestDiff(ctx, in.Owner, in.Repo, in.Index)
	if err != nil {
		return DiffPage{}, fmt.Errorf("get pull request diff err: %w", err)
	}
	if in.FilePath != "" {
		slice, found := diff.FileSlice(body, in.FilePath)
		if !found {
			return DiffPage{}, &DiffFileNotFoundError{Path: in.FilePath}
		}
		return DiffPage{Text: slice}, nil
	}
	c := s.ceiling(ctx)
	files := diff.Files(body)
	page, err := list.Run(ctx, list.NewRequest(list.Paging{Page: in.Page, Limit: in.Limit}, defaultDiffLimit), c,
		func(_ context.Context, p list.Paging) (list.Batch[string], error) {
			rows := list.Cut(files, p).Items
			return list.Batch[string]{Items: rows, Total: ptr.To(len(files))}, nil
		})
	if err != nil {
		return DiffPage{}, err
	}
	return DiffPage{
		Text:    strings.Join(page.Items, "\n"),
		Page:    page.Page,
		Limit:   page.Limit,
		Count:   page.Count,
		Files:   len(files),
		HasNext: page.HasNext,
	}, nil
}

// DiffPage is one page of a pull request's diff. Text holds whole files joined
// as the diff has them. Page, Limit and Count describe the page; Files is the
// number of files in the whole diff; HasNext reports that files follow. A
// file_path result has no page: Page and Limit are zero.
type DiffPage struct {
	Text    string
	Page    int
	Limit   int
	Count   int
	Files   int
	HasNext bool
}

// Reviews lists one page of the reviews of a pull request with its next marker.
func (s *Service) Reviews(ctx context.Context, in PagedPR) (list.Bounded[*model.PullReview], error) {
	c := s.ceiling(ctx)
	page, err := list.Run(ctx, list.Request{Page: in.Page, Limit: in.Limit, Def: defaultLimit}, c, func(ctx context.Context, p list.Paging) (list.Batch[*model.PullReview], error) {
		page, err := s.port.ListPullReviews(ctx, in.Owner, in.Repo, in.Index, Paging(p))
		if err != nil {
			return list.Batch[*model.PullReview]{}, fmt.Errorf("list pull reviews err: %w", err)
		}
		return list.Batch[*model.PullReview]{Items: page.Reviews}, nil
	})
	if err != nil {
		return list.Bounded[*model.PullReview]{}, err
	}
	return page, nil
}

// Review returns one review.
func (s *Service) Review(ctx context.Context, in ReviewRef) (*model.PullReview, error) {
	review, err := s.port.GetPullReview(ctx, in.Owner, in.Repo, in.Index, in.ID)
	if err != nil {
		return nil, fmt.Errorf("get pull review err: %w", err)
	}
	return review, nil
}

// ReviewComments returns one page of the inline comments of a review. Forgejo
// does not page this listing, so the whole list is read and cut here, and the
// grand total is the length of that list. The adapter decides whether to show
// it: the review-comment endpoint has never reported one.
func (s *Service) ReviewComments(ctx context.Context, in ReviewRef, p Paging) (list.Bounded[*model.PullReviewComment], error) {
	c := s.ceiling(ctx)
	page, err := list.Run(ctx, list.NewRequest(list.Paging(p), defaultLimit), c,
		func(ctx context.Context, q list.Paging) (list.Batch[*model.PullReviewComment], error) {
			all, err := s.port.ListPullReviewComments(ctx, in.Owner, in.Repo, in.Index, in.ID)
			if err != nil {
				return list.Batch[*model.PullReviewComment]{}, fmt.Errorf("list pull review comments err: %w", err)
			}
			rows := list.Cut(all, q).Items
			return list.Batch[*model.PullReviewComment]{Items: rows, Total: ptr.To(len(all))}, nil
		})
	if err != nil {
		return list.Bounded[*model.PullReviewComment]{}, err
	}
	return page, nil
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
