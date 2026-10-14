// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/list"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/split"
)

// AdminService runs the repository-administration use cases over an AdminPort.
// Build one per request from the port the request's credential selects; it
// holds no other state.
type AdminService struct{ port AdminPort }

// NewAdmin returns the administration use cases bound to port.
func NewAdmin(port AdminPort) *AdminService { return &AdminService{port: port} }

// The inputs below are what an MCP adapter has decoded from a tool call. The
// adapter applies a tool's defaults for a page or limit that was zero; the use
// cases validate what they are given.

const (
	// maxRepoTopics is Forgejo's topic limit per repository, and the page size
	// get_repo reads them with.
	maxRepoTopics = 25
	maxTopicLen   = 35

	defaultStatusesLimit = 30
	maxStatusesLimit     = 50
)

var topicNameRe = regexp.MustCompile(`^[a-z0-9][-.a-z0-9]*$`)

// CreateRepoInput creates a repository. Name is nil when the argument was not
// given, which is the one case the use case rejects.
type CreateRepoInput struct {
	Owner                                string
	Name                                 *string
	Description, IssueLabels, Gitignores string
	License, Readme, DefaultBranch       string
	Private, AutoInit, Template          bool
}

// ForkInput forks a repository. User and Repo are nil when not given.
type ForkInput struct {
	User, Repo         *string
	Organization, Name string
}

// EditRepoInput changes a repository's settings. Name and DefaultBranch that
// are empty count as not given, as they always have.
type EditRepoInput struct {
	Owner, Repo string
	Edit        RepoEdit
}

// CreateRepo creates a repository, owned by in.Owner when it is set.
func (s *AdminService) CreateRepo(ctx context.Context, in CreateRepoInput) (*model.Repository, error) {
	if in.Name == nil {
		return nil, errors.New("repository name is required")
	}
	repo, err := s.port.CreateRepo(ctx, in.Owner, NewRepo{
		Name: *in.Name, Description: in.Description, Private: in.Private,
		IssueLabels: in.IssueLabels, AutoInit: in.AutoInit, Template: in.Template,
		Gitignores: in.Gitignores, License: in.License, Readme: in.Readme,
		DefaultBranch: in.DefaultBranch,
	})
	if err != nil {
		return nil, fmt.Errorf("create repo err: %w", err)
	}
	return repo, nil
}

// ForkRepo forks a repository into the credential's user, or into an
// organization when one is given.
func (s *AdminService) ForkRepo(ctx context.Context, in ForkInput) error {
	if in.User == nil {
		return errors.New("user name is required")
	}
	if in.Repo == nil {
		return errors.New("repository name is required")
	}
	opt := ForkOptions{Organization: optional(in.Organization), Name: optional(in.Name)}
	if err := s.port.ForkRepo(ctx, *in.User, *in.Repo, opt); err != nil {
		return fmt.Errorf("fork repository error %w", err)
	}
	return nil
}

// ListMyRepos lists the repositories the credential's user owns, one page.
func (s *AdminService) ListMyRepos(ctx context.Context, p AdminPaging) ([]*model.Repository, error) {
	repos, err := s.port.ListMyRepos(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("list my repositories error: %w", err)
	}
	return repos, nil
}

// GetRepo returns a repository with its topics. A repository with no topics
// list, or one whose topic list Forgejo reports as missing, has an empty list.
func (s *AdminService) GetRepo(ctx context.Context, owner, repo string) (*model.RepositoryView, error) {
	read, err := s.ReadRepo(ctx, owner, repo, RepoReadOptions{Topics: true})
	if err != nil {
		return nil, err
	}
	return &model.RepositoryView{Repository: read.Repo, Topics: read.Topics}, nil
}

// EditRepo changes the settings given and returns the repository. It is an
// error to give none.
func (s *AdminService) EditRepo(ctx context.Context, in EditRepoInput) (*model.Repository, error) {
	if in.Owner == "" || in.Repo == "" {
		return nil, errors.New("owner and repo are required")
	}
	e := in.Edit
	if e.Name != nil && *e.Name == "" {
		e.Name = nil
	}
	if e.DefaultBranch != nil && *e.DefaultBranch == "" {
		e.DefaultBranch = nil
	}
	if !e.any() {
		return nil, errors.New("edit_repo: at least one setting field must be provided")
	}
	r, err := s.port.EditRepo(ctx, in.Owner, in.Repo, e)
	if err != nil {
		return nil, fmt.Errorf("edit repo: %w", err)
	}
	return r, nil
}

// ListRepoTopics returns one page of a repository's topics.
func (s *AdminService) ListRepoTopics(ctx context.Context, owner, repo string, p AdminPaging) (model.TopicPage, error) {
	if owner == "" || repo == "" {
		return model.TopicPage{}, errors.New("owner and repo are required")
	}
	c := ceiling(ctx, s.port)
	page, err := list.Run(ctx, list.Given(list.Paging(p), defaultListLimit, c), c,
		func(ctx context.Context, q list.Paging) (list.Batch[string], error) {
			topics, err := s.port.ListRepoTopics(ctx, owner, repo, AdminPaging(q))
			if err != nil {
				return list.Batch[string]{}, fmt.Errorf("list repo topics: %w", err)
			}
			return list.Batch[string]{Items: topics}, nil
		})
	if err != nil {
		return model.TopicPage{}, err
	}
	topics := page.Items
	if topics == nil {
		topics = []string{}
	}
	return model.TopicPage{Topics: topics, PageMeta: model.PageMetaOf(page)}, nil
}

// SetRepoTopics replaces every topic of a repository with the comma-separated
// names in raw. raw is nil when the argument was not given; an empty string
// clears them all. Names are lowercased and de-duplicated.
func (s *AdminService) SetRepoTopics(ctx context.Context, owner, repo string, raw *string) ([]string, error) {
	if owner == "" || repo == "" {
		return nil, errors.New("owner and repo are required")
	}
	if raw == nil {
		return nil, errors.New("topics is required")
	}
	topics, err := splitTopics(*raw)
	if err != nil {
		return nil, err
	}
	if err := s.port.SetRepoTopics(ctx, owner, repo, topics); err != nil {
		return nil, fmt.Errorf("set repo topics: %w", err)
	}
	return topics, nil
}

// AddRepoTopic adds one topic and returns its normalised name.
func (s *AdminService) AddRepoTopic(ctx context.Context, owner, repo, raw string) (string, error) {
	if owner == "" || repo == "" {
		return "", errors.New("owner and repo are required")
	}
	name, err := normalizeTopic(raw)
	if err != nil {
		return "", err
	}
	if err := s.port.AddRepoTopic(ctx, owner, repo, name); err != nil {
		return "", fmt.Errorf("add repo topic: %w", err)
	}
	return name, nil
}

// DeleteRepoTopic removes one topic and returns its normalised name.
func (s *AdminService) DeleteRepoTopic(ctx context.Context, owner, repo, raw string) (string, error) {
	if owner == "" || repo == "" {
		return "", errors.New("owner and repo are required")
	}
	name, err := normalizeTopic(raw)
	if err != nil {
		return "", err
	}
	if err := s.port.DeleteRepoTopic(ctx, owner, repo, name); err != nil {
		return "", fmt.Errorf("delete repo topic: %w", err)
	}
	return name, nil
}

// GetCommitStatuses returns one page of the statuses of a full 40-character
// commit SHA. A page below 1 is 1; a limit below 1 is the default, and above
// the maximum it is the maximum.
func (s *AdminService) GetCommitStatuses(ctx context.Context, owner, repo, sha string, page, limit int) (model.CommitStatuses, error) {
	read, err := s.ReadCommitStatuses(ctx, owner, repo, sha, page, limit)
	if err != nil {
		return model.CommitStatuses{}, err
	}
	return read.CommitStatuses, nil
}

// any reports whether an edit changes at least one setting.
func (e RepoEdit) any() bool {
	return e.Name != nil || e.Description != nil || e.Website != nil || e.DefaultBranch != nil ||
		e.Private != nil || e.Template != nil || e.Archived != nil ||
		e.HasIssues != nil || e.HasWiki != nil || e.HasPullRequests != nil ||
		e.HasProjects != nil || e.HasReleases != nil || e.HasPackages != nil || e.HasActions != nil
}

// optional is s as a pointer, or nil when it is empty.
func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func normalizeTopic(s string) (string, error) {
	name := strings.ToLower(strings.TrimSpace(s))
	if name == "" {
		return "", fmt.Errorf("topic name is empty")
	}
	if len(name) > maxTopicLen {
		return "", fmt.Errorf("topic %q exceeds %d characters", name, maxTopicLen)
	}
	if !topicNameRe.MatchString(name) {
		return "", fmt.Errorf("topic %q is invalid: must match ^[a-z0-9][-.a-z0-9]*$", name)
	}
	return name, nil
}

func splitTopics(s string) ([]string, error) {
	parts := split.CSV(s)
	out := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		name, err := normalizeTopic(part)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	if len(out) > maxRepoTopics {
		return nil, fmt.Errorf("at most %d topics per repository, got %d", maxRepoTopics, len(out))
	}
	return out, nil
}

// validateSHA accepts exactly 40 hexadecimal characters.
func validateSHA(sha string) error {
	if len(sha) != 40 {
		return fmt.Errorf("%w: sha must be exactly 40 hex characters, got %d", errInvalidParams, len(sha))
	}
	for _, c := range sha {
		isHex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
		if !isHex {
			return fmt.Errorf("%w: sha contains invalid character %q", errInvalidParams, c)
		}
	}
	return nil
}
