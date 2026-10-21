// SPDX-License-Identifier: GPL-3.0-or-later

// Package issue holds the issue use cases behind the issue, label and
// dependency tools. Like all of internal/app/ it is MCP-free and SDK-free
// (the depguard rules in .golangci.yml): use cases take typed inputs, reach
// Forgejo only through Port, and return repo-owned types from internal/app/model for an
// MCP adapter in internal/operation/issue to render.
package issue

import (
	"context"
	"time"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
)

// Port is the Forgejo issue, label and dependency domains as the use cases
// need them, in repo-owned types. internal/forge/issueadapter implements it over the
// Forgejo port (forge.Issues, forge.Labels, forge.Dependencies). Failures from
// Forgejo come back as ordinary errors for the use case to wrap; a 404 comes
// back as a *NotFoundError.
type Port interface {
	GetIssue(ctx context.Context, owner, repo string, index int64) (*model.Issue, error)
	ListIssues(ctx context.Context, owner, repo string, q IssueList) ([]*model.Issue, error)
	// SearchIssues searches across every repository of q.Owner and reports the
	// total when Forgejo gives one.
	SearchIssues(ctx context.Context, q SearchQuery) ([]*model.Issue, *int, error)
	// MaxResponseItems is the instance's pagination ceiling; ok is false when
	// it cannot be determined.
	MaxResponseItems(ctx context.Context) (limit int, ok bool)
	CreateIssue(ctx context.Context, owner, repo string, in NewIssue) (*model.Issue, error)
	EditIssue(ctx context.Context, owner, repo string, index int64, in IssueEdit) (*model.Issue, error)
	ReplaceIssueLabels(ctx context.Context, owner, repo string, index int64, labelIDs []int64) error
	ClearIssueLabels(ctx context.Context, owner, repo string, index int64) error
	AddIssueLabels(ctx context.Context, owner, repo string, index int64, labelIDs []int64) error
	RemoveIssueLabel(ctx context.Context, owner, repo string, index, labelID int64) error

	ListIssueComments(ctx context.Context, owner, repo string, index int64, c CommentList) ([]*model.Comment, error)
	GetIssueComment(ctx context.Context, owner, repo string, id int64) (*model.Comment, error)
	CreateIssueComment(ctx context.Context, owner, repo string, index int64, body string) (*model.Comment, error)
	EditIssueComment(ctx context.Context, owner, repo string, id int64, body string) (*model.Comment, error)
	DeleteIssueComment(ctx context.Context, owner, repo string, id int64) error
	ListRepoMilestones(ctx context.Context, owner, repo string, m MilestoneList) ([]*model.Milestone, error)

	ListRepoLabels(ctx context.Context, owner, repo string, p Paging) ([]*model.Label, error)
	GetRepoLabel(ctx context.Context, owner, repo string, id int64) (*model.Label, error)
	CreateRepoLabel(ctx context.Context, owner, repo string, in LabelNew) (*model.Label, error)
	EditRepoLabel(ctx context.Context, owner, repo string, id int64, in LabelEdit) (*model.Label, error)
	DeleteRepoLabel(ctx context.Context, owner, repo string, id int64) error
	ListOrgLabels(ctx context.Context, org string, p Paging) ([]*model.Label, error)
	// ListRepoLabelsPage and ListOrgLabelsPage are the label list reads: one
	// page, with what the server reported about rows past it.
	ListRepoLabelsPage(ctx context.Context, owner, repo string, p Paging) (LabelPage, error)
	ListOrgLabelsPage(ctx context.Context, org string, p Paging) (LabelPage, error)
	GetOrgLabel(ctx context.Context, org string, id int64) (*model.Label, error)
	CreateOrgLabel(ctx context.Context, org string, in LabelNew) (*model.Label, error)
	EditOrgLabel(ctx context.Context, org string, id int64, in LabelEdit) (*model.Label, error)
	DeleteOrgLabel(ctx context.Context, org string, id int64) error
	// ListOrgRepoNames pages the names of an organization's repositories. A
	// page past the end is empty.
	ListOrgRepoNames(ctx context.Context, org string, p Paging) ([]string, error)
	// CountIssuesLabelled counts the issues and pull requests of a repository
	// that carry the named label. The total is nil when Forgejo does not report
	// one.
	CountIssuesLabelled(ctx context.Context, owner, repo, label string) (*int, error)

	ListIssueDependencies(ctx context.Context, owner, repo string, index int64, p Paging) ([]*model.Issue, error)
	ListIssueDependents(ctx context.Context, owner, repo string, index int64, p Paging) ([]*model.Issue, error)
	AddIssueDependency(ctx context.Context, owner, repo string, index int64, dep Target) error
	RemoveIssueDependency(ctx context.Context, owner, repo string, index int64, dep Target) error
}

// Target names an issue, or a pull request, in a repository.
type Target struct {
	Owner, Repo string
	Index       int64
}

// Paging selects a page of a list. The use cases pass it on as given; the
// MCP adapter applies the tool's defaults.
type Paging struct{ Page, Limit int }

// IssueList filters and pages a repository's issue list. Milestones and Labels
// are already split into names or IDs; nil means no filter.
type IssueList struct {
	State, Type, Sort  string
	Milestones, Labels []string
	Paging
}

// SearchQuery is a search across an owner's repositories.
type SearchQuery struct {
	Owner, State, Type                          string
	Labels, Milestones                          []string
	Keyword, CreatedBy, AssignedBy, MentionedBy string
	Paging
}

// NewIssue is what opening an issue takes. Labels, when non-nil, are label
// IDs; Assignees, when non-nil, may be empty, which Forgejo reads as "none".
type NewIssue struct {
	Title, Body, Ref string
	Labels           []int64
	Assignees        []string
	Milestone        int64
}

// IssueEdit is a partial update of an issue. A nil field is left unchanged;
// a non-nil Assignees (even empty) replaces the assignees, and an empty Ref
// clears the branch link.
type IssueEdit struct {
	Title          string
	Body           *string
	Ref            *string
	Assignees      []string
	Milestone      *int64
	State          *string
	Deadline       *time.Time
	RemoveDeadline *bool
}

// CommentList pages an issue's comments, optionally within a time window.
type CommentList struct {
	Since, Before time.Time
	Paging
}

// MilestoneList filters and pages a repository's milestones.
type MilestoneList struct {
	State string
	Paging
}

// LabelNew is a label to create.
type LabelNew struct{ Name, Color, Description string }

// LabelEdit is a partial label update: a nil field is left alone. Tool names
// the tool the edit came from, for the error when nothing is supplied.
type LabelEdit struct {
	Tool                     string
	Name, Color, Description *string
}
