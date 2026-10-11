// SPDX-License-Identifier: GPL-3.0-or-later

// Package repoadminadapter implements the repository-administration use cases'
// port (app/repo.AdminPort) over the Forgejo port (forge.RepoAdmin and the
// commit-status method of forge.RepoFiles). It is the one place that translates
// between repo-owned types and SDK types for this domain: inputs become SDK
// options, SDK results become app/model values. Repositories are converted
// through their JSON form, which is exactly what a tool renders, so a field the
// repo-owned type shares by JSON name cannot drift in value.
package repoadminadapter

import (
	"context"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
	apprepo "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/repo"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/bridge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// Source is the part of the Forgejo port the admin use cases use. *fake.Forgejo
// and the SDK adapter both satisfy it.
type Source interface {
	forge.RepoAdmin
	forge.RepoFiles
}

// New returns the use cases' port over s.
func New(s Source) apprepo.AdminPort { return adapter{s: s} }

type adapter struct{ s Source }

var _ apprepo.AdminPort = adapter{}

func listOptions(p apprepo.AdminPaging) forgejo_sdk.ListOptions {
	return forgejo_sdk.ListOptions{Page: p.Page, PageSize: p.Limit}
}

func (a adapter) CreateRepo(ctx context.Context, owner string, in apprepo.NewRepo) (*model.Repository, error) {
	opt := forgejo_sdk.CreateRepoOption{
		Name:          in.Name,
		Description:   in.Description,
		Private:       in.Private,
		IssueLabels:   in.IssueLabels,
		AutoInit:      in.AutoInit,
		Template:      in.Template,
		Gitignores:    in.Gitignores,
		License:       in.License,
		Readme:        in.Readme,
		DefaultBranch: in.DefaultBranch,
	}
	var (
		r   *forgejo_sdk.Repository
		err error
	)
	if owner != "" {
		r, err = a.s.CreateOrgRepo(ctx, owner, opt)
	} else {
		r, err = a.s.CreateRepo(ctx, opt)
	}
	return bridge.Convert[*model.Repository](r, err)
}

func (a adapter) ForkRepo(ctx context.Context, owner, repo string, in apprepo.ForkOptions) error {
	_, err := a.s.ForkRepo(ctx, owner, repo, forgejo_sdk.CreateForkOption{
		Organization: in.Organization,
		Name:         in.Name,
	})
	return err
}

func (a adapter) ListMyRepos(ctx context.Context, p apprepo.AdminPaging) ([]*model.Repository, error) {
	repos, err := a.s.ListMyRepos(ctx, forgejo_sdk.ListReposOptions{ListOptions: listOptions(p)})
	return bridge.Convert[[]*model.Repository](repos, err)
}

func (a adapter) GetRepo(ctx context.Context, owner, repo string) (*model.Repository, error) {
	r, err := a.s.GetRepo(ctx, owner, repo)
	return bridge.Convert[*model.Repository](r, err)
}

func (a adapter) EditRepo(ctx context.Context, owner, repo string, in apprepo.RepoEdit) (*model.Repository, error) {
	r, err := a.s.EditRepo(ctx, owner, repo, forgejo_sdk.EditRepoOption{
		Name:            in.Name,
		Description:     in.Description,
		Website:         in.Website,
		DefaultBranch:   in.DefaultBranch,
		Private:         in.Private,
		Template:        in.Template,
		Archived:        in.Archived,
		HasIssues:       in.HasIssues,
		HasWiki:         in.HasWiki,
		HasPullRequests: in.HasPullRequests,
		HasProjects:     in.HasProjects,
		HasReleases:     in.HasReleases,
		HasPackages:     in.HasPackages,
		HasActions:      in.HasActions,
	})
	return bridge.Convert[*model.Repository](r, err)
}

func (a adapter) ListRepoTopics(ctx context.Context, owner, repo string, p apprepo.AdminPaging) ([]string, error) {
	topics, err := a.s.ListRepoTopics(ctx, owner, repo, forgejo_sdk.ListRepoTopicsOptions{ListOptions: listOptions(p)})
	if forge.IsNotFound(err) {
		return nil, &apprepo.NotFoundError{Err: err}
	}
	return topics, err
}

func (a adapter) SetRepoTopics(ctx context.Context, owner, repo string, topics []string) error {
	return a.s.SetRepoTopics(ctx, owner, repo, topics)
}

func (a adapter) AddRepoTopic(ctx context.Context, owner, repo, topic string) error {
	return a.s.AddRepoTopic(ctx, owner, repo, topic)
}

func (a adapter) DeleteRepoTopic(ctx context.Context, owner, repo, topic string) error {
	return a.s.DeleteRepoTopic(ctx, owner, repo, topic)
}

func (a adapter) ListCommitStatuses(ctx context.Context, owner, repo, sha string, p apprepo.AdminPaging) (apprepo.StatusPage, error) {
	res, err := a.s.ListCommitStatuses(ctx, owner, repo, sha, forgejo_sdk.ListStatusesOption{ListOptions: listOptions(p)})
	if err != nil {
		return apprepo.StatusPage{}, err
	}
	items := make([]model.StatusItem, 0, len(res.Statuses))
	for _, s := range res.Statuses {
		if s == nil {
			continue
		}
		items = append(items, statusItem(s))
	}
	return apprepo.StatusPage{Items: items, TotalCount: res.TotalCount}, nil
}

// statusItem maps one SDK status to its wire shape. The creation time is
// formatted to RFC 3339, as the tools have always rendered it.
func statusItem(s *forgejo_sdk.Status) model.StatusItem {
	return model.StatusItem{
		Context:     s.Context,
		State:       string(s.State),
		TargetURL:   s.TargetURL,
		Description: s.Description,
		CreatedAt:   s.Created.Format(time.RFC3339),
	}
}
