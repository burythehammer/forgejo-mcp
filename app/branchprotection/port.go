// SPDX-License-Identifier: GPL-3.0-or-later

// Package branchprotection holds the branch protection use cases behind the
// branch protection tools. Like all of app/ it is MCP-free and SDK-free
// (scripts/ci/check-app-boundary.sh): use cases take typed inputs, reach
// Forgejo only through Port, and return repo-owned types from app/model for an
// MCP adapter in operation/branchprotection to render.
package branchprotection

import (
	"context"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/model"
)

// Port is the Forgejo branch protection domain as the use cases need it, in
// repo-owned types. pkg/forge/branchprotectionadapter implements it over
// forge.BranchProtections. Failures from Forgejo come back as ordinary errors
// for the use case to wrap.
type Port interface {
	// MaxResponseItems is the instance's pagination ceiling; ok is false when it
	// cannot be determined, which is "unknown", never a ceiling of zero.
	MaxResponseItems(ctx context.Context) (limit int, ok bool)
	ListBranchProtections(ctx context.Context, owner, repo string, p Paging) ([]*model.BranchProtection, error)
	GetBranchProtection(ctx context.Context, owner, repo, rule string) (*model.BranchProtection, error)
	CreateBranchProtection(ctx context.Context, owner, repo string, in CreateInput) (*model.BranchProtection, error)
	EditBranchProtection(ctx context.Context, owner, repo, rule string, in EditInput) (*model.BranchProtection, error)
	DeleteBranchProtection(ctx context.Context, owner, repo, rule string) error
}

// Paging selects a page of a list.
type Paging struct{ Page, Limit int }

// CreateInput is a new branch protection rule. The zero value of every field
// is what Forgejo gets when the caller did not set it. A nil StatusCheckContexts
// is sent as null, as the create tool always did for an absent argument.
type CreateInput struct {
	BranchName, RuleName                                               string
	EnablePush, EnableStatusCheck                                      bool
	StatusCheckContexts                                                []string
	RequiredApprovals                                                  int64
	BlockOnOutdatedBranch, RequireSignedCommits, DismissStaleApprovals bool
}

// EditInput is a partial update: a nil field is left unchanged on the server.
type EditInput struct {
	EnablePush                  *bool
	EnablePushWhitelist         *bool
	PushWhitelistUsernames      *[]string
	EnableMergeWhitelist        *bool
	MergeWhitelistUsernames     *[]string
	EnableStatusCheck           *bool
	StatusCheckContexts         *[]string
	RequiredApprovals           *int64
	EnableApprovalsWhitelist    *bool
	ApprovalsWhitelistUsernames *[]string
	BlockOnOutdatedBranch       *bool
	DismissStaleApprovals       *bool
	RequireSignedCommits        *bool
}
