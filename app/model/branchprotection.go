// SPDX-License-Identifier: GPL-3.0-or-later

package model

import "time"

// BranchProtection is a repository's branch protection rule as the branch
// protection tools return it. Its fields, JSON names and order are those of the
// SDK's BranchProtection, so a rule renders the same bytes either way (the
// branchprotectionadapter round-trips through JSON and a shape test pins it).
type BranchProtection struct {
	BranchName                    string    `json:"branch_name"`
	RuleName                      string    `json:"rule_name"`
	EnablePush                    bool      `json:"enable_push"`
	EnablePushWhitelist           bool      `json:"enable_push_whitelist"`
	PushWhitelistUsernames        []string  `json:"push_whitelist_usernames"`
	PushWhitelistTeams            []string  `json:"push_whitelist_teams"`
	PushWhitelistDeployKeys       bool      `json:"push_whitelist_deploy_keys"`
	EnableMergeWhitelist          bool      `json:"enable_merge_whitelist"`
	MergeWhitelistUsernames       []string  `json:"merge_whitelist_usernames"`
	MergeWhitelistTeams           []string  `json:"merge_whitelist_teams"`
	EnableStatusCheck             bool      `json:"enable_status_check"`
	StatusCheckContexts           []string  `json:"status_check_contexts"`
	RequiredApprovals             int64     `json:"required_approvals"`
	EnableApprovalsWhitelist      bool      `json:"enable_approvals_whitelist"`
	ApprovalsWhitelistUsernames   []string  `json:"approvals_whitelist_username"`
	ApprovalsWhitelistTeams       []string  `json:"approvals_whitelist_teams"`
	BlockOnRejectedReviews        bool      `json:"block_on_rejected_reviews"`
	BlockOnOfficialReviewRequests bool      `json:"block_on_official_review_requests"`
	BlockOnOutdatedBranch         bool      `json:"block_on_outdated_branch"`
	DismissStaleApprovals         bool      `json:"dismiss_stale_approvals"`
	RequireSignedCommits          bool      `json:"require_signed_commits"`
	ProtectedFilePatterns         string    `json:"protected_file_patterns"`
	UnprotectedFilePatterns       string    `json:"unprotected_file_patterns"`
	Created                       time.Time `json:"created_at"`
	Updated                       time.Time `json:"updated_at"`
}

// BranchProtectionList is one page of a repository's branch protection rules,
// echoing the page so a caller can resume pagination.
type BranchProtectionList struct {
	Page              int                 `json:"page"`
	Limit             int                 `json:"limit"`
	Count             int                 `json:"count"`
	BranchProtections []*BranchProtection `json:"branch_protections"`
}
