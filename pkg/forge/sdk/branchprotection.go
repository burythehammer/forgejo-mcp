// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var _ forge.BranchProtections = (*Adapter)(nil)

func (a *Adapter) ListBranchProtections(_ context.Context, owner, repo string, opt forgejo_sdk.ListBranchProtectionsOptions) ([]*forgejo_sdk.BranchProtection, error) {
	bps, resp, err := a.client.ListBranchProtections(owner, repo, opt)
	return bps, statusErr(resp, err)
}

func (a *Adapter) GetBranchProtection(_ context.Context, owner, repo, rule string) (*forgejo_sdk.BranchProtection, error) {
	bp, resp, err := a.client.GetBranchProtection(owner, repo, rule)
	return bp, statusErr(resp, err)
}

func (a *Adapter) CreateBranchProtection(_ context.Context, owner, repo string, opt forgejo_sdk.CreateBranchProtectionOption) (*forgejo_sdk.BranchProtection, error) {
	bp, resp, err := a.client.CreateBranchProtection(owner, repo, opt)
	return bp, statusErr(resp, err)
}

func (a *Adapter) EditBranchProtection(_ context.Context, owner, repo, rule string, opt forgejo_sdk.EditBranchProtectionOption) (*forgejo_sdk.BranchProtection, error) {
	bp, resp, err := a.client.EditBranchProtection(owner, repo, rule, opt)
	return bp, statusErr(resp, err)
}

func (a *Adapter) DeleteBranchProtection(_ context.Context, owner, repo, rule string) error {
	resp, err := a.client.DeleteBranchProtection(owner, repo, rule)
	return statusErr(resp, err)
}
