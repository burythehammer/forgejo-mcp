// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"
	"errors"
	"fmt"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
)

// The read use cases below are the one read behind each repository, commit and
// commit-status entity. The get_repo, get_commit_statuses and list tools and the
// forgejo://repo, commit and status resources are thin adapters over them: each
// adapter decodes its input, asks for what it needs and encodes the result its
// own way. The options say which differences are intentional.

// RepoReadOptions says what a read of one repository carries beyond the
// repository itself. The zero value is the repository alone, which is what the
// forgejo://repo resource returns; get_repo asks for its topics too.
type RepoReadOptions struct {
	// Topics, when true, also lists the repository's topics. A repository whose
	// topic list Forgejo reports as missing has none.
	Topics bool
}

// RepoRead is one repository read with the options it asked for.
type RepoRead struct {
	Repo *model.Repository
	// Topics is nil unless the read asked for them, and then never nil.
	Topics []string
}

// ReadRepo returns one repository and, when opt asks for them, its topics.
func (s *AdminService) ReadRepo(ctx context.Context, owner, repo string, opt RepoReadOptions) (RepoRead, error) {
	if owner == "" || repo == "" {
		return RepoRead{}, errors.New("owner and repo are required")
	}
	r, err := s.port.GetRepo(ctx, owner, repo)
	if err != nil {
		return RepoRead{}, fmt.Errorf("get repo: %w", err)
	}
	read := RepoRead{Repo: r}
	if !opt.Topics {
		return read, nil
	}
	topics, err := s.port.ListRepoTopics(ctx, owner, repo, AdminPaging{Page: 1, Limit: maxRepoTopics})
	if err != nil {
		var nf *NotFoundError
		if !errors.As(err, &nf) {
			return RepoRead{}, fmt.Errorf("list repo topics: %w", err)
		}
		topics = nil
	}
	if topics == nil {
		topics = []string{}
	}
	read.Topics = topics
	return read, nil
}

// CommitStatusRead is one page of a commit's statuses.
type CommitStatusRead struct {
	model.CommitStatuses
}

// maxStatusPages bounds how many pages CommitStatusSummary reads. At
// maxStatusesLimit a page, a summary sees at most maxStatusPages*maxStatusesLimit
// statuses.
const maxStatusPages = 20

// CommitStatusSummary is every status of a commit, read page by page, with the
// combined state over all of them.
type CommitStatusSummary struct {
	// Statuses holds every status read, up to the read ceiling.
	Statuses []model.StatusItem
	// TotalCount is Forgejo's grand total, or the number read when Forgejo
	// reported none.
	TotalCount int
	// Complete is false when the ceiling stopped the read before the last status.
	Complete bool
	// State is AggregateState over Statuses when Complete, and "unknown" otherwise:
	// a state is never computed from a partial view.
	State string
}

// CommitStatusSummary reads all of a full 40-character commit SHA's statuses and
// aggregates their state. It never reports a state from a partial read.
func (s *AdminService) CommitStatusSummary(ctx context.Context, owner, repo, sha string) (CommitStatusSummary, error) {
	if owner == "" || repo == "" {
		return CommitStatusSummary{}, errors.New("owner and repo are required")
	}
	if err := validateSHA(sha); err != nil {
		return CommitStatusSummary{}, err
	}
	var all []model.StatusItem
	total, known, complete := 0, false, false
	for page := 1; page <= maxStatusPages; page++ {
		res, err := s.port.ListCommitStatuses(ctx, owner, repo, sha, AdminPaging{Page: page, Limit: maxStatusesLimit})
		if err != nil {
			return CommitStatusSummary{}, fmt.Errorf("get commit statuses: %w", err)
		}
		all = append(all, res.Items...)
		if res.TotalCount != nil {
			total, known = *res.TotalCount, true
		}
		if known {
			if len(all) >= total {
				complete = true
				break
			}
			if len(res.Items) == 0 {
				break
			}
		} else if len(res.Items) < maxStatusesLimit {
			complete = true
			break
		}
	}
	if !known {
		total = len(all)
	}
	sum := CommitStatusSummary{Statuses: all, TotalCount: total, Complete: complete, State: "unknown"}
	if complete {
		sum.State = AggregateState(all)
	}
	return sum, nil
}

// ReadCommitStatuses returns one page of a commit's per-context statuses. Its
// state is not aggregated here: use CommitStatusSummary for that. A page below 1 is 1; a limit below 1 is the default, and
// above the maximum it is the maximum.
func (s *AdminService) ReadCommitStatuses(ctx context.Context, owner, repo, sha string, page, limit int) (CommitStatusRead, error) {
	if owner == "" || repo == "" {
		return CommitStatusRead{}, errors.New("owner and repo are required")
	}
	if err := validateSHA(sha); err != nil {
		return CommitStatusRead{}, err
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = defaultStatusesLimit
	}
	if limit > maxStatusesLimit {
		limit = maxStatusesLimit
	}
	res, err := s.port.ListCommitStatuses(ctx, owner, repo, sha, AdminPaging{Page: page, Limit: limit})
	if err != nil {
		return CommitStatusRead{}, fmt.Errorf("get commit statuses: %w", err)
	}
	items := res.Items
	if items == nil {
		items = []model.StatusItem{}
	}
	return CommitStatusRead{
		CommitStatuses: model.CommitStatuses{
			SHA:        sha,
			Statuses:   items,
			Page:       page,
			Limit:      limit,
			Count:      len(items),
			TotalCount: res.TotalCount,
		},
	}, nil
}

// AggregateState is the combined state of a commit's statuses: failure when any
// is failure or error, else pending when any is pending, else success when all
// are success, else unknown. No statuses is unknown.
func AggregateState(statuses []model.StatusItem) string {
	if len(statuses) == 0 {
		return "unknown"
	}
	hasFailure, hasPending := false, false
	allSuccess := true
	for _, s := range statuses {
		switch s.State {
		case "failure", "error":
			hasFailure = true
			allSuccess = false
		case "pending":
			hasPending = true
			allSuccess = false
		case "success":
			// counted by allSuccess staying true
		default:
			allSuccess = false
		}
	}
	switch {
	case hasFailure:
		return "failure"
	case hasPending:
		return "pending"
	case allSuccess:
		return "success"
	default:
		return "unknown"
	}
}

// ReadCommit returns one commit by its full SHA, as the forgejo://repo commit
// resource reads it. The commit list tool reads commits in pages instead.
func (s *GitService) ReadCommit(ctx context.Context, owner, repo, sha string) (*model.Commit, error) {
	out, err := s.port.GetCommit(ctx, owner, repo, sha)
	if err != nil {
		return nil, fmt.Errorf("get commit err: %w", err)
	}
	return out, nil
}
