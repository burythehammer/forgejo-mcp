// SPDX-License-Identifier: GPL-3.0-or-later

package forge

import (
	"context"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// RepoAdmin is the repository-administration domain of the port: a repository's
// own record and settings, its topics, and its branches. Files, contents,
// commits and statuses are the RepoFiles port.
//
// Methods are named for what the repo tools and the repo resource need, and use
// SDK types as plain data. Failures from Forgejo are *StatusError.
type RepoAdmin interface {
	GetRepo(ctx context.Context, owner, repo string) (*forgejo_sdk.Repository, error)
	EditRepo(ctx context.Context, owner, repo string, opt forgejo_sdk.EditRepoOption) (*forgejo_sdk.Repository, error)

	ListRepoTopics(ctx context.Context, owner, repo string, opt forgejo_sdk.ListRepoTopicsOptions) ([]string, error)
	// SetRepoTopics replaces every topic of the repository.
	SetRepoTopics(ctx context.Context, owner, repo string, topics []string) error
	AddRepoTopic(ctx context.Context, owner, repo, topic string) error
	DeleteRepoTopic(ctx context.Context, owner, repo, topic string) error

	ListBranches(ctx context.Context, owner, repo string, opt forgejo_sdk.ListRepoBranchesOptions) ([]*forgejo_sdk.Branch, error)
	CreateBranch(ctx context.Context, owner, repo string, opt forgejo_sdk.CreateBranchOption) (*forgejo_sdk.Branch, error)
	// DeleteBranch reports whether Forgejo deleted the branch (HTTP 204).
	// A refusal (unknown repository or branch, the default branch) is
	// (false, nil), as the SDK reports it; the caller words the failure.
	DeleteBranch(ctx context.Context, owner, repo, branch string) (bool, error)
}
