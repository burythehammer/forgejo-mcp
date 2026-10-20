// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"net/http"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var _ forge.RepoAdmin = (*Adapter)(nil)

func (a *Adapter) GetRepo(_ context.Context, owner, repo string) (*forgejo_sdk.Repository, error) {
	r, resp, err := a.client.GetRepo(owner, repo)
	return r, statusErr(resp, err)
}

func (a *Adapter) EditRepo(_ context.Context, owner, repo string, opt forgejo_sdk.EditRepoOption) (*forgejo_sdk.Repository, error) {
	r, resp, err := a.client.EditRepo(owner, repo, opt)
	return r, statusErr(resp, err)
}

func (a *Adapter) ListRepoTopics(_ context.Context, owner, repo string, opt forgejo_sdk.ListRepoTopicsOptions) ([]string, error) {
	topics, resp, err := a.client.ListRepoTopics(owner, repo, opt)
	return topics, statusErr(resp, err)
}

func (a *Adapter) SetRepoTopics(_ context.Context, owner, repo string, topics []string) error {
	resp, err := a.client.SetRepoTopics(owner, repo, topics)
	return statusErr(resp, err)
}

func (a *Adapter) AddRepoTopic(_ context.Context, owner, repo, topic string) error {
	resp, err := a.client.AddRepoTopic(owner, repo, topic)
	return statusErr(resp, err)
}

func (a *Adapter) DeleteRepoTopic(_ context.Context, owner, repo, topic string) error {
	resp, err := a.client.DeleteRepoTopic(owner, repo, topic)
	return statusErr(resp, err)
}

func (a *Adapter) ListBranches(_ context.Context, owner, repo string, opt forgejo_sdk.ListRepoBranchesOptions) ([]*forgejo_sdk.Branch, error) {
	bs, resp, err := a.client.ListRepoBranches(owner, repo, opt)
	return bs, statusErr(resp, err)
}

func (a *Adapter) CreateBranch(_ context.Context, owner, repo string, opt forgejo_sdk.CreateBranchOption) (*forgejo_sdk.Branch, error) {
	b, resp, err := a.client.CreateBranch(owner, repo, opt)
	return b, statusErr(resp, err)
}

// DeleteBranch is raw so that a refusal keeps its status and Forgejo's message.
// The SDK's version gate for this endpoint is not repeated: a server too old to
// have it answers 404, which is reported as the refusal it is.
func (a *Adapter) DeleteBranch(ctx context.Context, owner, repo, branch string) (bool, error) {
	status, body, err := a.raw.doStatus(ctx, http.MethodDelete, forge.APIPath("repos", owner, repo, "branches", branch), nil)
	if err != nil {
		return false, err
	}
	if err := answered(status, body, http.StatusNoContent); err != nil {
		return false, err
	}
	return true, nil
}

func (a *Adapter) CreateRepo(_ context.Context, opt forgejo_sdk.CreateRepoOption) (*forgejo_sdk.Repository, error) {
	r, resp, err := a.client.CreateRepo(opt)
	return r, statusErr(resp, err)
}

func (a *Adapter) CreateOrgRepo(_ context.Context, org string, opt forgejo_sdk.CreateRepoOption) (*forgejo_sdk.Repository, error) {
	r, resp, err := a.client.CreateOrgRepo(org, opt)
	return r, statusErr(resp, err)
}

func (a *Adapter) ForkRepo(_ context.Context, owner, repo string, opt forgejo_sdk.CreateForkOption) (*forgejo_sdk.Repository, error) {
	r, resp, err := a.client.CreateFork(owner, repo, opt)
	return r, statusErr(resp, err)
}

func (a *Adapter) ListMyRepos(_ context.Context, opt forgejo_sdk.ListReposOptions) ([]*forgejo_sdk.Repository, error) {
	rs, resp, err := a.client.ListMyRepos(opt)
	return rs, statusErr(resp, err)
}
