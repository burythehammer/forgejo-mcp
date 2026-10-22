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
	ListRepoLabels(ctx context.Context, owner, repo string, page, limit int) ([]*Label, PageInfo, error)
	GetRepoLabel(ctx context.Context, owner, repo string, id int64) (*Label, error)
	CreateRepoLabel(ctx context.Context, owner, repo string, opt LabelCreate) (*Label, error)
	EditRepoLabel(ctx context.Context, owner, repo string, id int64, opt LabelPatch) (*Label, error)
	DeleteRepoLabel(ctx context.Context, owner, repo string, id int64) error

	// ListOrgLabels pages an organization's labels. An owner that is not an
	// organization (Forgejo answers 404) has none, which is an empty list, not
	// an error.
	ListOrgLabels(ctx context.Context, org string, page, limit int) ([]*Label, PageInfo, error)
	GetOrgLabel(ctx context.Context, org string, id int64) (*Label, error)
	CreateOrgLabel(ctx context.Context, org string, opt LabelCreate) (*Label, error)
	EditOrgLabel(ctx context.Context, org string, id int64, opt LabelPatch) (*Label, error)
	DeleteOrgLabel(ctx context.Context, org string, id int64) error

	// ListOrgRepoNames pages the names of the organization's repositories, for
	// counting an org label's usage. A page past the end is empty.
	ListOrgRepoNames(ctx context.Context, org string, page, limit int) ([]string, error)

	// AddIssueLabels adds labels, by ID, to the issue's existing ones.
	AddIssueLabels(ctx context.Context, owner, repo string, index int64, labelIDs []int64) error
	RemoveIssueLabel(ctx context.Context, owner, repo string, index, labelID int64) error
}

// Label is a Forgejo label, repository or organization. Exclusive is the flag
// the SDK's label type does not carry, so label calls read it from the raw
// response. The JSON names are Forgejo's, so the bridge converts a Label to the
// model by marshalling it.
type Label struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
	URL         string `json:"url"`
	Exclusive   bool   `json:"exclusive"`
}

// LabelCreate is the body that creates a label.
type LabelCreate struct {
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description,omitempty"`
	Exclusive   bool   `json:"exclusive"`
}

// LabelPatch is the body that edits a label. A nil field is left as it is, so
// an edit that omits Exclusive keeps the stored value.
type LabelPatch struct {
	Name        *string `json:"name,omitempty"`
	Color       *string `json:"color,omitempty"`
	Description *string `json:"description,omitempty"`
	Exclusive   *bool   `json:"exclusive,omitempty"`
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
