// SPDX-License-Identifier: GPL-3.0-or-later

// Package pull holds the pull-request use cases behind the pull tools. Like
// all of internal/app/ it is MCP-free and SDK-free (scripts/ci/check-app-boundary.sh):
// use cases take typed inputs, reach Forgejo only through Port, and return
// repo-owned types from internal/app/model for an MCP adapter in internal/operation/pull to
// render.
package pull

import (
	"context"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
)

// Port is the Forgejo pull-request domain as the use cases need it, in
// repo-owned types. internal/forge/pulladapter implements it over forge.Pulls (and
// so over the in-memory fake in tests and the real adapter in production).
// Failures from Forgejo come back as ordinary errors for the use case to wrap.
type Port interface {
	// MaxResponseItems is the instance's pagination ceiling; ok is false when it
	// cannot be determined, which is "unknown", never a ceiling of zero.
	MaxResponseItems(ctx context.Context) (limit int, ok bool)
	GetPullRequest(ctx context.Context, owner, repo string, index int64) (*model.PullRequest, error)
	ListPullRequests(ctx context.Context, owner, repo string, q ListQuery) ([]*model.PullRequest, error)
	CreatePullRequest(ctx context.Context, owner, repo string, in CreateInput) (*model.PullRequest, error)
	EditPullRequest(ctx context.Context, owner, repo string, index int64, in EditInput) (*model.PullRequest, error)
	MergePullRequest(ctx context.Context, owner, repo string, index int64, in MergeInput) (MergeOutcome, error)
	GetPullRequestDiff(ctx context.Context, owner, repo string, index int64) (string, error)
	ListPullRequestFiles(ctx context.Context, owner, repo string, index int64, p Paging) ([]*model.ChangedFile, error)

	ListPullRequestComments(ctx context.Context, owner, repo string, index int64, p Paging) ([]*model.Comment, error)
	ListPullReviews(ctx context.Context, owner, repo string, index int64, p Paging) ([]*model.PullReview, error)
	GetPullReview(ctx context.Context, owner, repo string, index, id int64) (*model.PullReview, error)
	ListPullReviewComments(ctx context.Context, owner, repo string, index, id int64) ([]*model.PullReviewComment, error)
	CreatePullReview(ctx context.Context, owner, repo string, index int64, in CreateReviewInput) (*model.PullReview, error)
	SubmitPullReview(ctx context.Context, owner, repo string, index, id int64, in SubmitReviewInput) (*model.PullReview, error)
	DismissPullReview(ctx context.Context, owner, repo string, index, id int64, message string) error
	DeletePullReview(ctx context.Context, owner, repo string, index, id int64) error
	CreateReviewRequests(ctx context.Context, owner, repo string, index int64, r Reviewers) error
	DeleteReviewRequests(ctx context.Context, owner, repo string, index int64, r Reviewers) error
}

// Paging selects a page of a list.
type Paging struct{ Page, Limit int }

// ListQuery filters and pages a pull-request list.
type ListQuery struct {
	State     string
	Sort      string
	Milestone int64
	Paging
}

// CreateInput is what opening a pull request takes.
type CreateInput struct{ Head, Base, Title, Body string }

// EditInput is a partial update: zero values (and a nil Body) are left alone.
type EditInput struct {
	Title    string
	Body     *string
	Base     string
	Assignee string
	// Milestone is a milestone ID; 0 leaves it alone.
	Milestone int64
}

// MergeInput is how a pull request is merged.
type MergeInput struct {
	Style, Title, Message                  string
	DeleteBranch, Force, WhenChecksSucceed bool
}

// MergeOutcome is Forgejo's answer to a merge. A merge Forgejo declines is not
// a transport error: Merged is false and StatusCode says why.
type MergeOutcome struct {
	Merged     bool
	StatusCode int
	// Message is Forgejo's explanation of a declined merge, or "".
	Message string
}

// ReviewComment is one inline comment of a new review. Its JSON tags are the
// shape the create_pull_review tool accepts in its comments argument.
type ReviewComment struct {
	Path       string `json:"path"`
	Body       string `json:"body"`
	OldLineNum int64  `json:"old_position"`
	NewLineNum int64  `json:"new_position"`
}

// CreateReviewInput is a new review with optional inline comments.
type CreateReviewInput struct {
	State, Body string
	Comments    []ReviewComment
}

// SubmitReviewInput submits a pending review.
type SubmitReviewInput struct{ State, Body string }

// Reviewers names the users and teams of a review request.
type Reviewers struct{ Users, Teams []string }
