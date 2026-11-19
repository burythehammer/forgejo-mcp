// SPDX-License-Identifier: GPL-3.0-or-later

// Package branchprotectionadapter implements the branch protection use cases'
// port (internal/app/branchprotection.Port) over the Forgejo port
// (forge.BranchProtections). It is the one place that translates between
// repo-owned types and SDK types for the branch protection domain: inputs
// become SDK options, SDK results become internal/app/model values. Results are
// converted through their JSON form, which is exactly what a tool renders, so
// a field the repo-owned type shares by JSON name cannot drift in value.
package branchprotectionadapter

import (
	"context"

	appbp "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/branchprotection"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/model"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge/bridge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// Source is the part of the Forgejo port the branch protection domain uses: its
// rule methods and the instance's pagination ceiling.
type Source interface {
	forge.BranchProtections
	forge.Ceilings
}

// New returns the use cases' port over p.
func New(p Source) appbp.Port { return adapter{p: p} }

type adapter struct{ p Source }

// MaxResponseItems reads the instance's pagination ceiling through the source.
func (a adapter) MaxResponseItems(ctx context.Context) (int, bool) {
	return a.p.MaxResponseItems(ctx)
}

var _ appbp.Port = adapter{}

// deref returns the value a set pointer holds, or nil when it is unset.
func deref[T any](p *T) T {
	var zero T
	if p == nil {
		return zero
	}
	return *p
}

func (a adapter) ListBranchProtections(ctx context.Context, owner, repo string, p appbp.Paging) ([]*model.BranchProtection, error) {
	bps, err := a.p.ListBranchProtections(ctx, owner, repo, forgejo_sdk.ListBranchProtectionsOptions{
		ListOptions: forgejo_sdk.ListOptions{Page: p.Page, PageSize: p.Limit},
	})
	return bridge.Convert[[]*model.BranchProtection](bps, err)
}

func (a adapter) GetBranchProtection(ctx context.Context, owner, repo, rule string) (*model.BranchProtection, error) {
	bp, err := a.p.GetBranchProtection(ctx, owner, repo, rule)
	return bridge.Convert[*model.BranchProtection](bp, err)
}

func (a adapter) CreateBranchProtection(ctx context.Context, owner, repo string, in appbp.CreateInput) (*model.BranchProtection, error) {
	bp, err := a.p.CreateBranchProtection(ctx, owner, repo, forgejo_sdk.CreateBranchProtectionOption{
		BranchName:            in.BranchName,
		RuleName:              in.RuleName,
		EnablePush:            in.EnablePush,
		EnableStatusCheck:     in.EnableStatusCheck,
		StatusCheckContexts:   in.StatusCheckContexts,
		RequiredApprovals:     in.RequiredApprovals,
		BlockOnOutdatedBranch: in.BlockOnOutdatedBranch,
		RequireSignedCommits:  in.RequireSignedCommits,
		DismissStaleApprovals: in.DismissStaleApprovals,
	})
	return bridge.Convert[*model.BranchProtection](bp, err)
}

func (a adapter) EditBranchProtection(ctx context.Context, owner, repo, rule string, in appbp.EditInput) (*model.BranchProtection, error) {
	bp, err := a.p.EditBranchProtection(ctx, owner, repo, rule, forgejo_sdk.EditBranchProtectionOption{
		EnablePush:                  in.EnablePush,
		EnablePushWhitelist:         in.EnablePushWhitelist,
		PushWhitelistUsernames:      deref(in.PushWhitelistUsernames),
		EnableMergeWhitelist:        in.EnableMergeWhitelist,
		MergeWhitelistUsernames:     deref(in.MergeWhitelistUsernames),
		EnableStatusCheck:           in.EnableStatusCheck,
		StatusCheckContexts:         deref(in.StatusCheckContexts),
		RequiredApprovals:           in.RequiredApprovals,
		EnableApprovalsWhitelist:    in.EnableApprovalsWhitelist,
		ApprovalsWhitelistUsernames: deref(in.ApprovalsWhitelistUsernames),
		BlockOnOutdatedBranch:       in.BlockOnOutdatedBranch,
		DismissStaleApprovals:       in.DismissStaleApprovals,
		RequireSignedCommits:        in.RequireSignedCommits,
	})
	return bridge.Convert[*model.BranchProtection](bp, err)
}

func (a adapter) DeleteBranchProtection(ctx context.Context, owner, repo, rule string) error {
	return a.p.DeleteBranchProtection(ctx, owner, repo, rule)
}
