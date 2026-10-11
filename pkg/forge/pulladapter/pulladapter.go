// SPDX-License-Identifier: GPL-3.0-or-later

// Package pulladapter implements the pull use cases' port (app/pull.Port) over
// the Forgejo port (forge.Pulls). It is the one place that translates between
// repo-owned types and SDK types for the pull domain: inputs become SDK
// options, SDK results become app/model values. Results are converted through
// their JSON form, which is exactly what a tool renders, so a field the
// repo-owned type shares by JSON name cannot drift in value.
package pulladapter

import (
	"context"
	"encoding/json"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
	apppull "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/pull"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/ptr"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// New returns the use cases' port over p.
func New(p forge.Pulls) apppull.Port { return adapter{p: p} }

type adapter struct{ p forge.Pulls }

var _ apppull.Port = adapter{}

// convert maps an SDK value onto its repo-owned twin through JSON.
func convert[T any](from any, err error) (T, error) {
	var to T
	if err != nil {
		return to, err
	}
	b, err := json.Marshal(from)
	if err != nil {
		return to, err
	}
	err = json.Unmarshal(b, &to)
	return to, err
}

func listOptions(p apppull.Paging) forgejo_sdk.ListOptions {
	return forgejo_sdk.ListOptions{Page: p.Page, PageSize: p.Limit}
}

func (a adapter) GetPullRequest(ctx context.Context, owner, repo string, index int64) (*model.PullRequest, error) {
	pr, err := a.p.GetPullRequest(ctx, owner, repo, index)
	return convert[*model.PullRequest](pr, err)
}

func (a adapter) ListPullRequests(ctx context.Context, owner, repo string, q apppull.ListQuery) ([]*model.PullRequest, error) {
	prs, err := a.p.ListPullRequests(ctx, owner, repo, forgejo_sdk.ListPullRequestsOptions{
		ListOptions: listOptions(q.Paging),
		State:       forgejo_sdk.StateType(q.State),
		Sort:        q.Sort,
		Milestone:   q.Milestone,
	})
	return convert[[]*model.PullRequest](prs, err)
}

func (a adapter) CreatePullRequest(ctx context.Context, owner, repo string, in apppull.CreateInput) (*model.PullRequest, error) {
	pr, err := a.p.CreatePullRequest(ctx, owner, repo, forgejo_sdk.CreatePullRequestOption{
		Head: in.Head, Base: in.Base, Title: in.Title, Body: in.Body,
	})
	return convert[*model.PullRequest](pr, err)
}

func (a adapter) EditPullRequest(ctx context.Context, owner, repo string, index int64, in apppull.EditInput) (*model.PullRequest, error) {
	opt := forgejo_sdk.EditPullRequestOption{Title: in.Title, Base: in.Base, Assignee: in.Assignee, Milestone: in.Milestone}
	if in.Body != nil {
		opt.Body = ptr.To(*in.Body)
	}
	pr, err := a.p.EditPullRequest(ctx, owner, repo, index, opt)
	return convert[*model.PullRequest](pr, err)
}

func (a adapter) MergePullRequest(ctx context.Context, owner, repo string, index int64, in apppull.MergeInput) (apppull.MergeOutcome, error) {
	res, err := a.p.MergePullRequest(ctx, owner, repo, index, forgejo_sdk.MergePullRequestOption{
		Style:                  forgejo_sdk.MergeStyle(in.Style),
		Title:                  in.Title,
		Message:                in.Message,
		DeleteBranchAfterMerge: in.DeleteBranch,
		ForceMerge:             in.Force,
		MergeWhenChecksSucceed: in.WhenChecksSucceed,
	})
	return apppull.MergeOutcome{Merged: res.Merged, StatusCode: res.StatusCode}, err
}

func (a adapter) GetPullRequestDiff(ctx context.Context, owner, repo string, index int64) (string, error) {
	b, err := a.p.GetPullRequestDiff(ctx, owner, repo, index)
	return string(b), err
}

func (a adapter) ListPullRequestFiles(ctx context.Context, owner, repo string, index int64, p apppull.Paging) ([]*model.ChangedFile, error) {
	files, err := a.p.ListPullRequestFiles(ctx, owner, repo, index, forgejo_sdk.ListPullRequestFilesOptions{ListOptions: listOptions(p)})
	return convert[[]*model.ChangedFile](files, err)
}

func (a adapter) ListPullReviews(ctx context.Context, owner, repo string, index int64, p apppull.Paging) ([]*model.PullReview, error) {
	reviews, err := a.p.ListPullReviews(ctx, owner, repo, index, forgejo_sdk.ListPullReviewsOptions{ListOptions: listOptions(p)})
	return convert[[]*model.PullReview](reviews, err)
}

func (a adapter) GetPullReview(ctx context.Context, owner, repo string, index, id int64) (*model.PullReview, error) {
	review, err := a.p.GetPullReview(ctx, owner, repo, index, id)
	return convert[*model.PullReview](review, err)
}

func (a adapter) ListPullReviewComments(ctx context.Context, owner, repo string, index, id int64) ([]*model.PullReviewComment, error) {
	comments, err := a.p.ListPullReviewComments(ctx, owner, repo, index, id)
	return convert[[]*model.PullReviewComment](comments, err)
}

func (a adapter) CreatePullReview(ctx context.Context, owner, repo string, index int64, in apppull.CreateReviewInput) (*model.PullReview, error) {
	opt := forgejo_sdk.CreatePullReviewOptions{State: forgejo_sdk.ReviewStateType(in.State), Body: in.Body}
	for _, c := range in.Comments {
		opt.Comments = append(opt.Comments, forgejo_sdk.CreatePullReviewComment{
			Path: c.Path, Body: c.Body, OldLineNum: c.OldLineNum, NewLineNum: c.NewLineNum,
		})
	}
	review, err := a.p.CreatePullReview(ctx, owner, repo, index, opt)
	return convert[*model.PullReview](review, err)
}

func (a adapter) SubmitPullReview(ctx context.Context, owner, repo string, index, id int64, in apppull.SubmitReviewInput) (*model.PullReview, error) {
	review, err := a.p.SubmitPullReview(ctx, owner, repo, index, id, forgejo_sdk.SubmitPullReviewOptions{
		State: forgejo_sdk.ReviewStateType(in.State), Body: in.Body,
	})
	return convert[*model.PullReview](review, err)
}

func (a adapter) DismissPullReview(ctx context.Context, owner, repo string, index, id int64, message string) error {
	return a.p.DismissPullReview(ctx, owner, repo, index, id, forgejo_sdk.DismissPullReviewOptions{Message: message})
}

func (a adapter) DeletePullReview(ctx context.Context, owner, repo string, index, id int64) error {
	return a.p.DeletePullReview(ctx, owner, repo, index, id)
}

func (a adapter) CreateReviewRequests(ctx context.Context, owner, repo string, index int64, r apppull.Reviewers) error {
	return a.p.CreateReviewRequests(ctx, owner, repo, index, requestOptions(r))
}

func (a adapter) DeleteReviewRequests(ctx context.Context, owner, repo string, index int64, r apppull.Reviewers) error {
	return a.p.DeleteReviewRequests(ctx, owner, repo, index, requestOptions(r))
}

func requestOptions(r apppull.Reviewers) forgejo_sdk.PullReviewRequestOptions {
	return forgejo_sdk.PullReviewRequestOptions{Reviewers: r.Users, TeamReviewers: r.Teams}
}
