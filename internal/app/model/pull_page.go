// SPDX-License-Identifier: GPL-3.0-or-later

package model

// PullRequestPage is the response of list_repo_pull_requests: one page of
// pull requests with the continuation signal of docs/design/output-bounding.md.
type PullRequestPage struct {
	PullRequests []*PullRequest `json:"pull_requests"`
	PageMeta
}

// PullReviewPage is the response of list_pull_reviews.
type PullReviewPage struct {
	Reviews []*PullReview `json:"reviews"`
	PageMeta
}

// PullReviewCommentPage is the response of list_pull_review_comments.
type PullReviewCommentPage struct {
	Comments []*PullReviewComment `json:"comments"`
	PageMeta
}

// ChangedFilePage is the response of list_pull_request_files.
type ChangedFilePage struct {
	Files []*ChangedFile `json:"files"`
	PageMeta
}
