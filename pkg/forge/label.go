// SPDX-License-Identifier: GPL-3.0-or-later

package forge

import (
	"context"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// Labels is the label domain of the port: repository and organization label
// catalogues, and adding or removing labels on an issue. (Replacing or
// clearing an issue's whole label set is on Issues.)
//
// Failures from Forgejo are *StatusError; validation failures are plain errors.
type Labels interface {
	ListRepoLabels(ctx context.Context, owner, repo string, opt forgejo_sdk.ListLabelsOptions) ([]*forgejo_sdk.Label, PageInfo, error)
	GetRepoLabel(ctx context.Context, owner, repo string, id int64) (*forgejo_sdk.Label, error)
	CreateRepoLabel(ctx context.Context, owner, repo string, opt forgejo_sdk.CreateLabelOption) (*forgejo_sdk.Label, error)
	EditRepoLabel(ctx context.Context, owner, repo string, id int64, opt forgejo_sdk.EditLabelOption) (*forgejo_sdk.Label, error)
	DeleteRepoLabel(ctx context.Context, owner, repo string, id int64) error

	// ListOrgLabels pages an organization's labels. An owner that is not an
	// organization (Forgejo answers 404) has none, which is an empty list, not
	// an error.
	ListOrgLabels(ctx context.Context, org string, page, limit int) ([]*forgejo_sdk.Label, PageInfo, error)
	GetOrgLabel(ctx context.Context, org string, id int64) (*forgejo_sdk.Label, error)
	CreateOrgLabel(ctx context.Context, org string, opt forgejo_sdk.CreateLabelOption) (*forgejo_sdk.Label, error)
	EditOrgLabel(ctx context.Context, org string, id int64, opt forgejo_sdk.EditLabelOption) (*forgejo_sdk.Label, error)
	DeleteOrgLabel(ctx context.Context, org string, id int64) error

	// ListOrgRepoNames pages the names of the organization's repositories, for
	// counting an org label's usage. A page past the end is empty.
	ListOrgRepoNames(ctx context.Context, org string, page, limit int) ([]string, error)

	// AddIssueLabels adds labels, by ID, to the issue's existing ones.
	AddIssueLabels(ctx context.Context, owner, repo string, index int64, labelIDs []int64) error
	RemoveIssueLabel(ctx context.Context, owner, repo string, index, labelID int64) error
}

// IssueRef names an issue in any repository.
type IssueRef struct {
	Owner, Repo string
	Index       int64
}

// Dependencies is the issue-dependency domain of the port.
//
// Listing is lenient the way Forgejo is: an issue or repository that answers
// 404 (including a repository with dependencies disabled) lists as empty.
type Dependencies interface {
	// ListIssueDependencies lists the issues the given issue depends on.
	ListIssueDependencies(ctx context.Context, owner, repo string, index int64, page, limit int) ([]*forgejo_sdk.Issue, error)
	// ListIssueDependents lists the issues that depend on the given issue.
	ListIssueDependents(ctx context.Context, owner, repo string, index int64, page, limit int) ([]*forgejo_sdk.Issue, error)
	// AddIssueDependency makes the issue depend on dep, which may be in
	// another repository.
	AddIssueDependency(ctx context.Context, owner, repo string, index int64, dep IssueRef) error
	RemoveIssueDependency(ctx context.Context, owner, repo string, index int64, dep IssueRef) error
}
