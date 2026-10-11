// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"

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

func (a *Adapter) DeleteBranch(_ context.Context, owner, repo, branch string) (bool, error) {
	ok, resp, err := a.client.DeleteRepoBranch(owner, repo, branch)
	return ok, statusErr(resp, err)
}
