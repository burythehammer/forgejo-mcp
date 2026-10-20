// SPDX-License-Identifier: GPL-3.0-or-later

package fake

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"sync"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var _ forge.RepoAdmin = (*Forgejo)(nil)

// repoAdminState holds what the RepoAdmin domain adds to a repository: its
// settings, topics and branches. Whether a repository exists is the aggregate's
// business (AddRepo); a repository's admin record is created on first use with
// Forgejo's defaults (default branch "main", which exists).
type repoAdminState struct {
	mu    sync.Mutex
	repos map[string]*adminRepo // key "owner/repo"
}

type adminRepo struct {
	repo     *forgejo_sdk.Repository
	topics   []string
	branches []*forgejo_sdk.Branch
}

func (f *Forgejo) repoExists(owner, repo string) bool {
	f.pulls.mu.Lock()
	defer f.pulls.mu.Unlock()
	_, ok := f.pulls.repos[owner+"/"+repo]
	return ok
}

// adminRepo returns the record for owner/repo, creating it on first use. The
// caller holds f.admin.mu.
func (f *Forgejo) adminRepo(owner, repo string) (*adminRepo, error) {
	if r, ok := f.admin.repos[owner+"/"+repo]; ok {
		return r, nil
	}
	if !f.repoExists(owner, repo) {
		return nil, notFound("repository")
	}
	if f.admin.repos == nil {
		f.admin.repos = map[string]*adminRepo{}
	}
	r := &adminRepo{
		repo: &forgejo_sdk.Repository{
			Name:            repo,
			FullName:        owner + "/" + repo,
			Owner:           user(owner),
			DefaultBranch:   "main",
			HasIssues:       true,
			HasWiki:         true,
			HasPullRequests: true,
		},
		branches: []*forgejo_sdk.Branch{{Name: "main"}},
	}
	f.admin.repos[owner+"/"+repo] = r
	return r, nil
}

func (r *adminRepo) branch(name string) int {
	for i, b := range r.branches {
		if b.Name == name {
			return i
		}
	}
	return -1
}

func (f *Forgejo) GetRepo(_ context.Context, owner, repo string) (*forgejo_sdk.Repository, error) {
	f.admin.mu.Lock()
	defer f.admin.mu.Unlock()
	r, err := f.adminRepo(owner, repo)
	if err != nil {
		return nil, err
	}
	c := *r.repo
	return &c, nil
}

func (f *Forgejo) EditRepo(_ context.Context, owner, repo string, opt forgejo_sdk.EditRepoOption) (*forgejo_sdk.Repository, error) {
	f.admin.mu.Lock()
	defer f.admin.mu.Unlock()
	r, err := f.adminRepo(owner, repo)
	if err != nil {
		return nil, err
	}
	if opt.DefaultBranch != nil && r.branch(*opt.DefaultBranch) < 0 {
		return nil, statusErr(http.StatusUnprocessableEntity, "default branch does not exist")
	}
	if opt.Name != nil && *opt.Name != repo {
		if _, taken := f.admin.repos[owner+"/"+*opt.Name]; taken || f.repoExists(owner, *opt.Name) {
			return nil, statusErr(http.StatusUnprocessableEntity, "repository name already taken")
		}
		delete(f.admin.repos, owner+"/"+repo)
		f.admin.repos[owner+"/"+*opt.Name] = r
		f.moveRepo(owner, repo, *opt.Name)
		r.repo.Name = *opt.Name
		r.repo.FullName = owner + "/" + *opt.Name
	}
	setS := func(dst *string, v *string) {
		if v != nil {
			*dst = *v
		}
	}
	setB := func(dst *bool, v *bool) {
		if v != nil {
			*dst = *v
		}
	}
	setS(&r.repo.Description, opt.Description)
	setS(&r.repo.Website, opt.Website)
	setS(&r.repo.DefaultBranch, opt.DefaultBranch)
	setB(&r.repo.Private, opt.Private)
	setB(&r.repo.Template, opt.Template)
	setB(&r.repo.Archived, opt.Archived)
	setB(&r.repo.HasIssues, opt.HasIssues)
	setB(&r.repo.HasWiki, opt.HasWiki)
	setB(&r.repo.HasPullRequests, opt.HasPullRequests)
	setB(&r.repo.HasProjects, opt.HasProjects)
	setB(&r.repo.HasReleases, opt.HasReleases)
	setB(&r.repo.HasPackages, opt.HasPackages)
	setB(&r.repo.HasActions, opt.HasActions)
	c := *r.repo
	return &c, nil
}

// moveRepo renames the aggregate's existence record, so a renamed repository
// is found under its new name only. Lock order: admin, then pulls.
func (f *Forgejo) moveRepo(owner, from, to string) {
	f.pulls.mu.Lock()
	defer f.pulls.mu.Unlock()
	if rs, ok := f.pulls.repos[owner+"/"+from]; ok {
		delete(f.pulls.repos, owner+"/"+from)
		f.pulls.repos[owner+"/"+to] = rs
	}
}

func (f *Forgejo) ListRepoTopics(_ context.Context, owner, repo string, opt forgejo_sdk.ListRepoTopicsOptions) ([]string, error) {
	f.admin.mu.Lock()
	defer f.admin.mu.Unlock()
	r, err := f.adminRepo(owner, repo)
	if err != nil {
		return nil, err
	}
	lo, hi := page(opt.ListOptions, len(r.topics))
	return append([]string{}, r.topics[lo:hi]...), nil
}

func (f *Forgejo) SetRepoTopics(_ context.Context, owner, repo string, topics []string) error {
	f.admin.mu.Lock()
	defer f.admin.mu.Unlock()
	r, err := f.adminRepo(owner, repo)
	if err != nil {
		return err
	}
	r.topics = append([]string{}, topics...)
	return nil
}

func (f *Forgejo) AddRepoTopic(_ context.Context, owner, repo, topic string) error {
	f.admin.mu.Lock()
	defer f.admin.mu.Unlock()
	r, err := f.adminRepo(owner, repo)
	if err != nil {
		return err
	}
	for _, t := range r.topics {
		if t == topic {
			return nil
		}
	}
	r.topics = append(r.topics, topic)
	return nil
}

func (f *Forgejo) DeleteRepoTopic(_ context.Context, owner, repo, topic string) error {
	f.admin.mu.Lock()
	defer f.admin.mu.Unlock()
	r, err := f.adminRepo(owner, repo)
	if err != nil {
		return err
	}
	for i, t := range r.topics {
		if t == topic {
			r.topics = append(r.topics[:i], r.topics[i+1:]...)
			return nil
		}
	}
	return notFound("topic")
}

func (f *Forgejo) ListBranches(_ context.Context, owner, repo string, opt forgejo_sdk.ListRepoBranchesOptions) ([]*forgejo_sdk.Branch, error) {
	f.admin.mu.Lock()
	defer f.admin.mu.Unlock()
	r, err := f.adminRepo(owner, repo)
	if err != nil {
		return nil, err
	}
	lo, hi := page(opt.ListOptions, len(r.branches))
	out := make([]*forgejo_sdk.Branch, 0, hi-lo)
	for _, b := range r.branches[lo:hi] {
		c := *b
		out = append(out, &c)
	}
	return out, nil
}

func (f *Forgejo) CreateBranch(_ context.Context, owner, repo string, opt forgejo_sdk.CreateBranchOption) (*forgejo_sdk.Branch, error) {
	f.admin.mu.Lock()
	defer f.admin.mu.Unlock()
	r, err := f.adminRepo(owner, repo)
	if err != nil {
		return nil, err
	}
	from := opt.OldBranchName
	if from == "" {
		from = r.repo.DefaultBranch
	}
	if r.branch(from) < 0 {
		return nil, notFound("source branch")
	}
	if r.branch(opt.BranchName) >= 0 {
		return nil, statusErr(http.StatusConflict, "branch already exists")
	}
	b := &forgejo_sdk.Branch{Name: opt.BranchName}
	r.branches = append(r.branches, b)
	c := *b
	return &c, nil
}

// DeleteBranch mirrors the SDK: a branch Forgejo deletes is true; a refusal
// (unknown repository or branch, the default branch) is an error carrying the
// status and Forgejo's message, and nothing is deleted.
func (f *Forgejo) DeleteBranch(_ context.Context, owner, repo, branch string) (bool, error) {
	if err := f.refused("DeleteBranch"); err != nil {
		return false, err
	}
	f.admin.mu.Lock()
	defer f.admin.mu.Unlock()
	r, err := f.adminRepo(owner, repo)
	if err != nil {
		return false, err
	}
	i := r.branch(branch)
	if i < 0 {
		return false, statusErr(http.StatusNotFound, "branch does not exist")
	}
	if branch == r.repo.DefaultBranch {
		return false, statusErr(http.StatusForbidden, "branch is the default branch and cannot be deleted")
	}
	r.branches = append(r.branches[:i], r.branches[i+1:]...)
	return true, nil
}

// newRepo registers owner/repo as existing, with the settings opt asks for.
// It is refused when the repository already exists.
func (f *Forgejo) newRepo(owner, name, desc string, private bool, def string) (*forgejo_sdk.Repository, error) {
	if name == "" {
		return nil, statusErr(http.StatusUnprocessableEntity, "repository name is required")
	}
	if f.repoExists(owner, name) {
		return nil, statusErr(http.StatusConflict, "repository already exists")
	}
	f.AddRepo(owner, name)
	f.admin.mu.Lock()
	defer f.admin.mu.Unlock()
	r, err := f.adminRepo(owner, name)
	if err != nil {
		return nil, err
	}
	r.repo.Description = desc
	r.repo.Private = private
	if def != "" && def != "main" {
		r.repo.DefaultBranch = def
		r.branches = []*forgejo_sdk.Branch{{Name: def}}
	}
	c := *r.repo
	return &c, nil
}

func (f *Forgejo) CreateRepo(_ context.Context, opt forgejo_sdk.CreateRepoOption) (*forgejo_sdk.Repository, error) {
	return f.newRepo(Poster, opt.Name, opt.Description, opt.Private, opt.DefaultBranch)
}

func (f *Forgejo) CreateOrgRepo(_ context.Context, org string, opt forgejo_sdk.CreateRepoOption) (*forgejo_sdk.Repository, error) {
	if !f.orgExists(org) {
		return nil, notFound("organization")
	}
	return f.newRepo(org, opt.Name, opt.Description, opt.Private, opt.DefaultBranch)
}

func (f *Forgejo) ForkRepo(ctx context.Context, owner, repo string, opt forgejo_sdk.CreateForkOption) (*forgejo_sdk.Repository, error) {
	src, err := f.GetRepo(ctx, owner, repo)
	if err != nil {
		return nil, err
	}
	target := Poster
	if opt.Organization != nil && *opt.Organization != "" {
		target = *opt.Organization
		if !f.orgExists(target) {
			return nil, notFound("organization")
		}
	}
	name := repo
	if opt.Name != nil && *opt.Name != "" {
		name = *opt.Name
	}
	return f.newRepo(target, name, src.Description, src.Private, src.DefaultBranch)
}

func (f *Forgejo) ListMyRepos(_ context.Context, opt forgejo_sdk.ListReposOptions) ([]*forgejo_sdk.Repository, error) {
	f.pulls.mu.Lock()
	var names []string
	for k := range f.pulls.repos {
		if o, n, ok := strings.Cut(k, "/"); ok && o == Poster {
			names = append(names, n)
		}
	}
	f.pulls.mu.Unlock()
	sort.Strings(names)
	lo, hi := page(opt.ListOptions, len(names))
	out := []*forgejo_sdk.Repository{}
	for _, n := range names[lo:hi] {
		r, err := f.GetRepo(context.Background(), Poster, n)
		if err == nil {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *Forgejo) orgExists(name string) bool {
	f.orgs.mu.Lock()
	defer f.orgs.mu.Unlock()
	_, err := f.orgs.find(name)
	return err == nil
}
