// SPDX-License-Identifier: GPL-3.0-or-later

package forge

import (
	"context"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// Issues is the issue domain of the port: issues, their comments, the issue
// label assignment that update_issue needs, and repository milestones.
//
// Label and milestone catalogues, and issue dependencies, are other ports.
// Failures from Forgejo are *StatusError; validation failures (an empty title)
// are plain errors.
type Issues interface {
	GetIssue(ctx context.Context, owner, repo string, index int64) (*forgejo_sdk.Issue, error)
	ListRepoIssues(ctx context.Context, owner, repo string, opt ListIssuesOptions) ([]*forgejo_sdk.Issue, PageInfo, error)
	// SearchIssues searches across every repository of opt.Owner.
	SearchIssues(ctx context.Context, opt forgejo_sdk.ListIssueOption) ([]*forgejo_sdk.Issue, PageInfo, error)
	CreateIssue(ctx context.Context, owner, repo string, opt forgejo_sdk.CreateIssueOption) (*forgejo_sdk.Issue, error)
	EditIssue(ctx context.Context, owner, repo string, index int64, opt forgejo_sdk.EditIssueOption) (*forgejo_sdk.Issue, error)
	// ReplaceIssueLabels sets the issue's whole label set to the given label IDs.
	ReplaceIssueLabels(ctx context.Context, owner, repo string, index int64, labelIDs []int64) error
	ClearIssueLabels(ctx context.Context, owner, repo string, index int64) error

	// ListIssueComments returns an issue's (or pull request's) thread.
	ListIssueComments(ctx context.Context, owner, repo string, index int64, opt forgejo_sdk.ListIssueCommentOptions) ([]*forgejo_sdk.Comment, PageInfo, error)
	GetIssueComment(ctx context.Context, owner, repo string, id int64) (*forgejo_sdk.Comment, error)
	CreateIssueComment(ctx context.Context, owner, repo string, index int64, body string) (*forgejo_sdk.Comment, error)
	EditIssueComment(ctx context.Context, owner, repo string, id int64, body string) (*forgejo_sdk.Comment, error)
	DeleteIssueComment(ctx context.Context, owner, repo string, id int64) error

	ListRepoMilestones(ctx context.Context, owner, repo string, opt forgejo_sdk.ListMilestoneOption) ([]*forgejo_sdk.Milestone, error)

	// MaxResponseItems is the instance's pagination ceiling; ok is false when
	// it cannot be determined, which is "unknown", never a ceiling of zero.
	MaxResponseItems(ctx context.Context) (limit int, ok bool)
}

// ListIssuesOptions are the SDK's issue-list options plus Sort, which the
// endpoint accepts but the SDK cannot send. Sort is one of Forgejo's values
// (latest, oldest, nearduedate, ...); empty is the server's default.
type ListIssuesOptions struct {
	forgejo_sdk.ListIssueOption
	Sort string
}

// PageInfo is what a list answer says about the rest of the list.
type PageInfo struct {
	// HasNext is true when the server advertised a further page.
	HasNext bool
	// TotalCount is the number of matching rows across all pages, valid only
	// when TotalKnown (the server may not report it).
	TotalCount int
	TotalKnown bool
}
