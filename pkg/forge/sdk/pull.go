// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var _ forge.Pulls = (*Adapter)(nil)

func (a *Adapter) GetPullRequest(_ context.Context, owner, repo string, index int64) (*forgejo_sdk.PullRequest, error) {
	pr, resp, err := a.client.GetPullRequest(owner, repo, index)
	return pr, statusErr(resp, err)
}

func (a *Adapter) ListPullRequests(_ context.Context, owner, repo string, opt forgejo_sdk.ListPullRequestsOptions) ([]*forgejo_sdk.PullRequest, error) {
	prs, resp, err := a.client.ListRepoPullRequests(owner, repo, opt)
	return prs, statusErr(resp, err)
}

func (a *Adapter) CreatePullRequest(_ context.Context, owner, repo string, opt forgejo_sdk.CreatePullRequestOption) (*forgejo_sdk.PullRequest, error) {
	pr, resp, err := a.client.CreatePullRequest(owner, repo, opt)
	return pr, statusErr(resp, err)
}

func (a *Adapter) EditPullRequest(_ context.Context, owner, repo string, index int64, opt forgejo_sdk.EditPullRequestOption) (*forgejo_sdk.PullRequest, error) {
	pr, resp, err := a.client.EditPullRequest(owner, repo, index, opt)
	return pr, statusErr(resp, err)
}

func (a *Adapter) MergePullRequest(_ context.Context, owner, repo string, index int64, opt forgejo_sdk.MergePullRequestOption) (forge.MergeResult, error) {
	merged, resp, err := a.client.MergePullRequest(owner, repo, index, opt)
	if err != nil {
		return forge.MergeResult{}, statusErr(resp, err)
	}
	res := forge.MergeResult{Merged: merged}
	if resp != nil && resp.Response != nil {
		res.StatusCode = resp.StatusCode
	}
	return res, nil
}

func (a *Adapter) GetPullRequestDiff(_ context.Context, owner, repo string, index int64) ([]byte, error) {
	diff, resp, err := a.client.GetPullRequestDiff(owner, repo, index, forgejo_sdk.PullRequestDiffOptions{})
	return diff, statusErr(resp, err)
}

func (a *Adapter) ListPullRequestFiles(_ context.Context, owner, repo string, index int64, opt forgejo_sdk.ListPullRequestFilesOptions) ([]*forgejo_sdk.ChangedFile, error) {
	files, resp, err := a.client.ListPullRequestFiles(owner, repo, index, opt)
	return files, statusErr(resp, err)
}

func (a *Adapter) ListPullRequestComments(_ context.Context, owner, repo string, index int64, opt forgejo_sdk.ListOptions) ([]*forgejo_sdk.Comment, error) {
	comments, resp, err := a.client.ListIssueComments(owner, repo, index, forgejo_sdk.ListIssueCommentOptions{ListOptions: opt})
	return comments, statusErr(resp, err)
}

func (a *Adapter) ListPullReviews(_ context.Context, owner, repo string, index int64, opt forgejo_sdk.ListPullReviewsOptions) ([]*forgejo_sdk.PullReview, error) {
	reviews, resp, err := a.client.ListPullReviews(owner, repo, index, opt)
	return reviews, statusErr(resp, err)
}

func (a *Adapter) GetPullReview(_ context.Context, owner, repo string, index, id int64) (*forgejo_sdk.PullReview, error) {
	review, resp, err := a.client.GetPullReview(owner, repo, index, id)
	return review, statusErr(resp, err)
}

func (a *Adapter) ListPullReviewComments(_ context.Context, owner, repo string, index, id int64) ([]*forgejo_sdk.PullReviewComment, error) {
	comments, resp, err := a.client.ListPullReviewComments(owner, repo, index, id)
	return comments, statusErr(resp, err)
}

func (a *Adapter) CreatePullReview(_ context.Context, owner, repo string, index int64, opt forgejo_sdk.CreatePullReviewOptions) (*forgejo_sdk.PullReview, error) {
	review, resp, err := a.client.CreatePullReview(owner, repo, index, opt)
	return review, statusErr(resp, err)
}

func (a *Adapter) SubmitPullReview(_ context.Context, owner, repo string, index, id int64, opt forgejo_sdk.SubmitPullReviewOptions) (*forgejo_sdk.PullReview, error) {
	review, resp, err := a.client.SubmitPullReview(owner, repo, index, id, opt)
	return review, statusErr(resp, err)
}

func (a *Adapter) DismissPullReview(_ context.Context, owner, repo string, index, id int64, opt forgejo_sdk.DismissPullReviewOptions) error {
	resp, err := a.client.DismissPullReview(owner, repo, index, id, opt)
	return statusErr(resp, err)
}

func (a *Adapter) DeletePullReview(_ context.Context, owner, repo string, index, id int64) error {
	resp, err := a.client.DeletePullReview(owner, repo, index, id)
	return statusErr(resp, err)
}

func (a *Adapter) CreateReviewRequests(_ context.Context, owner, repo string, index int64, opt forgejo_sdk.PullReviewRequestOptions) error {
	resp, err := a.client.CreateReviewRequests(owner, repo, index, opt)
	return statusErr(resp, err)
}

func (a *Adapter) DeleteReviewRequests(_ context.Context, owner, repo string, index int64, opt forgejo_sdk.PullReviewRequestOptions) error {
	resp, err := a.client.DeleteReviewRequests(owner, repo, index, opt)
	return statusErr(resp, err)
}
