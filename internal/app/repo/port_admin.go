// SPDX-License-Identifier: GPL-3.0-or-later

// Package repo holds the repository use cases. Like all of internal/app/ it is MCP-free
// and SDK-free (the depguard rules in .golangci.yml): use cases take typed inputs,
// reach Forgejo only through a port, and return repo-owned types from internal/app/model
// for an MCP adapter in internal/operation/repo to render.
package repo

import (
	"context"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/model"
)

// AdminPort is the repository-administration part of Forgejo as the admin use
// cases need it: a repository's record and settings, its topics, and the
// commit statuses of a commit. internal/forge/repoadminadapter implements it over
// the Forgejo port. A 404 from Forgejo on a topic list comes back as a
// forge.ErrNotFound; other failures are ordinary errors for the use case to wrap.
type AdminPort interface {
	// MaxResponseItems is the instance's pagination ceiling; ok is false when it
	// cannot be determined, which is "unknown", never a ceiling of zero.
	MaxResponseItems(ctx context.Context) (limit int, ok bool)
	// CreateRepo creates a repository owned by owner, or by the credential's
	// user when owner is empty.
	CreateRepo(ctx context.Context, owner string, in NewRepo) (*model.Repository, error)
	// ForkRepo forks owner/repo into the credential's user, or into
	// in.Organization when it is set.
	ForkRepo(ctx context.Context, owner, repo string, in ForkOptions) error
	// ListMyRepos lists the repositories the credential's user owns.
	ListMyRepos(ctx context.Context, p AdminPaging) ([]*model.Repository, error)

	GetRepo(ctx context.Context, owner, repo string) (*model.Repository, error)
	EditRepo(ctx context.Context, owner, repo string, in RepoEdit) (*model.Repository, error)

	ListRepoTopics(ctx context.Context, owner, repo string, p AdminPaging) ([]string, error)
	// SetRepoTopics replaces every topic of the repository.
	SetRepoTopics(ctx context.Context, owner, repo string, topics []string) error
	AddRepoTopic(ctx context.Context, owner, repo, topic string) error
	DeleteRepoTopic(ctx context.Context, owner, repo, topic string) error

	// ListCommitStatuses is one page of a commit's per-context statuses. The
	// adapter drops nil entries and converts each status to a StatusItem.
	ListCommitStatuses(ctx context.Context, owner, repo, sha string, p AdminPaging) (StatusPage, error)
}

// AdminPaging selects a page of a list. The use cases pass it on as given; the
// MCP adapter applies the tool's defaults.
type AdminPaging struct{ Page, Limit int }

// NewRepo is what creating a repository takes. Fields are passed to Forgejo as
// given.
type NewRepo struct {
	Name, Description, IssueLabels, Gitignores, License, Readme, DefaultBranch string
	Private, AutoInit, Template                                                bool
}

// ForkOptions are the optional parts of a fork. A nil field is not sent.
type ForkOptions struct {
	Organization *string
	Name         *string
}

// RepoEdit is a partial update of a repository's settings. A nil field is left
// unchanged.
type RepoEdit struct {
	Name, Description, Website, DefaultBranch *string
	Private, Template, Archived               *bool
	HasIssues, HasWiki, HasPullRequests       *bool
	HasProjects, HasReleases, HasPackages     *bool
	HasActions                                *bool
}

// StatusPage is one page of a commit's statuses, with Forgejo's total when it
// reported one.
type StatusPage struct {
	Items      []model.StatusItem
	TotalCount *int
}
