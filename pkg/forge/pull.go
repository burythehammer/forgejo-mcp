// SPDX-License-Identifier: GPL-3.0-or-later

package forge

import (
	"context"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// Pulls is the pull-request domain of the port: pull requests, their reviews
// and review requests, their diff and files, and the discussion thread.
//
// Methods are named for what the pull tools and resources need, and use SDK
// types as plain data. Failures from Forgejo are *StatusError; validation
// failures (for example an empty review body) are plain errors.
type Pulls interface {
	GetPullRequest(ctx context.Context, owner, repo string, index int64) (*forgejo_sdk.PullRequest, error)
	ListPullRequests(ctx context.Context, owner, repo string, opt forgejo_sdk.ListPullRequestsOptions) ([]*forgejo_sdk.PullRequest, error)
	CreatePullRequest(ctx context.Context, owner, repo string, opt forgejo_sdk.CreatePullRequestOption) (*forgejo_sdk.PullRequest, error)
	EditPullRequest(ctx context.Context, owner, repo string, index int64, opt forgejo_sdk.EditPullRequestOption) (*forgejo_sdk.PullRequest, error)
	MergePullRequest(ctx context.Context, owner, repo string, index int64, opt forgejo_sdk.MergePullRequestOption) (MergeResult, error)

	// GetPullRequestDiff returns the unified diff of the whole pull request.
	GetPullRequestDiff(ctx context.Context, owner, repo string, index int64) ([]byte, error)
	ListPullRequestFiles(ctx context.Context, owner, repo string, index int64, opt forgejo_sdk.ListPullRequestFilesOptions) ([]*forgejo_sdk.ChangedFile, error)

	// ListPullRequestComments returns the discussion thread, which Forgejo
	// keeps on the issue-comments endpoint.
	ListPullRequestComments(ctx context.Context, owner, repo string, index int64, opt forgejo_sdk.ListOptions) ([]*forgejo_sdk.Comment, error)

	ListPullReviews(ctx context.Context, owner, repo string, index int64, opt forgejo_sdk.ListPullReviewsOptions) ([]*forgejo_sdk.PullReview, error)
	GetPullReview(ctx context.Context, owner, repo string, index, id int64) (*forgejo_sdk.PullReview, error)
	ListPullReviewComments(ctx context.Context, owner, repo string, index, id int64) ([]*forgejo_sdk.PullReviewComment, error)
	CreatePullReview(ctx context.Context, owner, repo string, index int64, opt forgejo_sdk.CreatePullReviewOptions) (*forgejo_sdk.PullReview, error)
	SubmitPullReview(ctx context.Context, owner, repo string, index, id int64, opt forgejo_sdk.SubmitPullReviewOptions) (*forgejo_sdk.PullReview, error)
	DismissPullReview(ctx context.Context, owner, repo string, index, id int64, opt forgejo_sdk.DismissPullReviewOptions) error
	DeletePullReview(ctx context.Context, owner, repo string, index, id int64) error

	// Review requests show up in ListPullReviews as REQUEST_REVIEW reviews.
	CreateReviewRequests(ctx context.Context, owner, repo string, index int64, opt forgejo_sdk.PullReviewRequestOptions) error
	DeleteReviewRequests(ctx context.Context, owner, repo string, index int64, opt forgejo_sdk.PullReviewRequestOptions) error
}

// MergeResult is Forgejo's answer to a merge request. A merge that Forgejo
// declines (not mergeable, already merged) is not an error at the transport
// level: Merged is false and StatusCode says why, which the tool reports.
type MergeResult struct {
	// Merged is true when Forgejo merged the pull request (HTTP 200).
	Merged bool
	// StatusCode is the HTTP status of the answer; 201 means the merge was
	// scheduled for when checks succeed.
	StatusCode int
	// Message is Forgejo's own explanation of a declined merge, from the
	// "message" field of a JSON answer. It is empty when the merge went through
	// or was scheduled, and when a declined answer had no such JSON message.
	Message string
}
