// SPDX-License-Identifier: GPL-3.0-or-later

package fake

import (
	"context"
	"errors"
	"net/http"
	"sync"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var _ forge.BranchProtections = (*Forgejo)(nil)

type bpState struct {
	mu    sync.Mutex
	rules map[string][]*forgejo_sdk.BranchProtection // key "owner/repo", creation order
}

// settingsRepoExists reports whether AddRepo made owner/repo exist.
func (f *Forgejo) settingsRepoExists(owner, repo string) bool {
	f.pulls.mu.Lock()
	defer f.pulls.mu.Unlock()
	_, ok := f.pulls.repos[owner+"/"+repo]
	return ok
}

func (f *Forgejo) bpRules(owner, repo string) ([]*forgejo_sdk.BranchProtection, error) {
	if !f.settingsRepoExists(owner, repo) {
		return nil, notFound("repository")
	}
	return f.bps.rules[owner+"/"+repo], nil
}

func (s *bpState) find(rules []*forgejo_sdk.BranchProtection, rule string) (int, error) {
	for i, r := range rules {
		if r.RuleName == rule {
			return i, nil
		}
	}
	return -1, notFound("branch protection")
}

func copyBP(bp *forgejo_sdk.BranchProtection) *forgejo_sdk.BranchProtection {
	c := *bp
	return &c
}

func (f *Forgejo) ListBranchProtections(_ context.Context, owner, repo string, opt forgejo_sdk.ListBranchProtectionsOptions) ([]*forgejo_sdk.BranchProtection, error) {
	f.bps.mu.Lock()
	defer f.bps.mu.Unlock()
	rules, err := f.bpRules(owner, repo)
	if err != nil {
		return nil, err
	}
	lo, hi := page(opt.ListOptions, len(rules))
	out := make([]*forgejo_sdk.BranchProtection, 0, hi-lo)
	for _, r := range rules[lo:hi] {
		out = append(out, copyBP(r))
	}
	return out, nil
}

func (f *Forgejo) GetBranchProtection(_ context.Context, owner, repo, rule string) (*forgejo_sdk.BranchProtection, error) {
	f.bps.mu.Lock()
	defer f.bps.mu.Unlock()
	rules, err := f.bpRules(owner, repo)
	if err != nil {
		return nil, err
	}
	i, err := f.bps.find(rules, rule)
	if err != nil {
		return nil, err
	}
	return copyBP(rules[i]), nil
}

func (f *Forgejo) CreateBranchProtection(_ context.Context, owner, repo string, opt forgejo_sdk.CreateBranchProtectionOption) (*forgejo_sdk.BranchProtection, error) {
	f.bps.mu.Lock()
	defer f.bps.mu.Unlock()
	rules, err := f.bpRules(owner, repo)
	if err != nil {
		return nil, err
	}
	name := opt.RuleName
	if name == "" {
		name = opt.BranchName
	}
	if name == "" {
		return nil, errors.New("branch_name or rule_name is required")
	}
	if _, err := f.bps.find(rules, name); err == nil {
		return nil, statusErr(http.StatusForbidden, "branch protection already exists")
	}
	now := f.Now()
	bp := &forgejo_sdk.BranchProtection{
		BranchName:                    opt.BranchName,
		RuleName:                      name,
		EnablePush:                    opt.EnablePush,
		EnablePushWhitelist:           opt.EnablePushWhitelist,
		PushWhitelistUsernames:        opt.PushWhitelistUsernames,
		PushWhitelistTeams:            opt.PushWhitelistTeams,
		PushWhitelistDeployKeys:       opt.PushWhitelistDeployKeys,
		EnableMergeWhitelist:          opt.EnableMergeWhitelist,
		MergeWhitelistUsernames:       opt.MergeWhitelistUsernames,
		MergeWhitelistTeams:           opt.MergeWhitelistTeams,
		EnableStatusCheck:             opt.EnableStatusCheck,
		StatusCheckContexts:           opt.StatusCheckContexts,
		RequiredApprovals:             opt.RequiredApprovals,
		EnableApprovalsWhitelist:      opt.EnableApprovalsWhitelist,
		ApprovalsWhitelistUsernames:   opt.ApprovalsWhitelistUsernames,
		ApprovalsWhitelistTeams:       opt.ApprovalsWhitelistTeams,
		BlockOnRejectedReviews:        opt.BlockOnRejectedReviews,
		BlockOnOfficialReviewRequests: opt.BlockOnOfficialReviewRequests,
		BlockOnOutdatedBranch:         opt.BlockOnOutdatedBranch,
		DismissStaleApprovals:         opt.DismissStaleApprovals,
		RequireSignedCommits:          opt.RequireSignedCommits,
		ProtectedFilePatterns:         opt.ProtectedFilePatterns,
		UnprotectedFilePatterns:       opt.UnprotectedFilePatterns,
		Created:                       now,
		Updated:                       now,
	}
	if f.bps.rules == nil {
		f.bps.rules = map[string][]*forgejo_sdk.BranchProtection{}
	}
	f.bps.rules[owner+"/"+repo] = append(rules, bp)
	return copyBP(bp), nil
}

func setIf[T any](dst *T, src *T) {
	if src != nil {
		*dst = *src
	}
}

func setSlice(dst *[]string, src []string) {
	if src != nil {
		*dst = src
	}
}

func (f *Forgejo) EditBranchProtection(_ context.Context, owner, repo, rule string, opt forgejo_sdk.EditBranchProtectionOption) (*forgejo_sdk.BranchProtection, error) {
	f.bps.mu.Lock()
	defer f.bps.mu.Unlock()
	rules, err := f.bpRules(owner, repo)
	if err != nil {
		return nil, err
	}
	i, err := f.bps.find(rules, rule)
	if err != nil {
		return nil, err
	}
	bp := rules[i]
	setIf(&bp.EnablePush, opt.EnablePush)
	setIf(&bp.EnablePushWhitelist, opt.EnablePushWhitelist)
	setSlice(&bp.PushWhitelistUsernames, opt.PushWhitelistUsernames)
	setSlice(&bp.PushWhitelistTeams, opt.PushWhitelistTeams)
	setIf(&bp.PushWhitelistDeployKeys, opt.PushWhitelistDeployKeys)
	setIf(&bp.EnableMergeWhitelist, opt.EnableMergeWhitelist)
	setSlice(&bp.MergeWhitelistUsernames, opt.MergeWhitelistUsernames)
	setSlice(&bp.MergeWhitelistTeams, opt.MergeWhitelistTeams)
	setIf(&bp.EnableStatusCheck, opt.EnableStatusCheck)
	setSlice(&bp.StatusCheckContexts, opt.StatusCheckContexts)
	setIf(&bp.RequiredApprovals, opt.RequiredApprovals)
	setIf(&bp.EnableApprovalsWhitelist, opt.EnableApprovalsWhitelist)
	setSlice(&bp.ApprovalsWhitelistUsernames, opt.ApprovalsWhitelistUsernames)
	setSlice(&bp.ApprovalsWhitelistTeams, opt.ApprovalsWhitelistTeams)
	setIf(&bp.BlockOnRejectedReviews, opt.BlockOnRejectedReviews)
	setIf(&bp.BlockOnOfficialReviewRequests, opt.BlockOnOfficialReviewRequests)
	setIf(&bp.BlockOnOutdatedBranch, opt.BlockOnOutdatedBranch)
	setIf(&bp.DismissStaleApprovals, opt.DismissStaleApprovals)
	setIf(&bp.RequireSignedCommits, opt.RequireSignedCommits)
	setIf(&bp.ProtectedFilePatterns, opt.ProtectedFilePatterns)
	setIf(&bp.UnprotectedFilePatterns, opt.UnprotectedFilePatterns)
	bp.Updated = f.Now()
	return copyBP(bp), nil
}

func (f *Forgejo) DeleteBranchProtection(_ context.Context, owner, repo, rule string) error {
	f.bps.mu.Lock()
	defer f.bps.mu.Unlock()
	rules, err := f.bpRules(owner, repo)
	if err != nil {
		return err
	}
	i, err := f.bps.find(rules, rule)
	if err != nil {
		return err
	}
	f.bps.rules[owner+"/"+repo] = append(rules[:i:i], rules[i+1:]...)
	return nil
}
