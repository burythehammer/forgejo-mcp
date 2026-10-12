// SPDX-License-Identifier: GPL-3.0-or-later

package branchprotection

import (
	"context"
	"fmt"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/split"
)

// Service runs the branch protection use cases over a Port. Build one per
// request from the port the request's credential selects; it holds no other
// state.
type Service struct{ port Port }

// New returns the use cases bound to port.
func New(port Port) *Service { return &Service{port: port} }

// RuleRef names one branch protection rule of a repository.
type RuleRef struct{ Owner, Repo, Rule string }

// ListRequest lists a repository's rules. Page and Limit are the numbers as
// the caller gave them: zero means page 1 and 100 rules, and a fraction is
// truncated after that default is applied.
type ListRequest struct {
	Owner, Repo string
	Page, Limit float64
}

// CreateRequest creates a rule. StatusCheckContexts is a comma-separated list,
// nil when the caller did not pass it. RequiredApprovals is truncated to a
// whole number.
type CreateRequest struct {
	Owner, Repo, BranchName, RuleName string
	EnablePush, EnableStatusCheck     bool
	StatusCheckContexts               *string
	RequiredApprovals                 float64
	BlockOnOutdatedBranch             bool
	RequireSignedCommits              bool
	DismissStaleApprovals             bool
}

// EditRequest changes a rule. A nil field is not changed. The list fields are
// comma-separated, and a non-nil empty string clears the list.
type EditRequest struct {
	RuleRef
	EnablePush                  *bool
	EnablePushWhitelist         *bool
	PushWhitelistUsernames      *string
	EnableMergeWhitelist        *bool
	MergeWhitelistUsernames     *string
	EnableStatusCheck           *bool
	StatusCheckContexts         *string
	RequiredApprovals           *float64
	EnableApprovalsWhitelist    *bool
	ApprovalsWhitelistUsernames *string
	BlockOnOutdatedBranch       *bool
	DismissStaleApprovals       *bool
	RequireSignedCommits        *bool
}

// List returns one page of a repository's rules.
func (s *Service) List(ctx context.Context, r ListRequest) (model.BranchProtectionList, error) {
	if r.Owner == "" || r.Repo == "" {
		return model.BranchProtectionList{}, fmt.Errorf("owner and repo are required")
	}
	page := r.Page
	if page == 0 {
		page = 1
	}
	limit := r.Limit
	if limit == 0 {
		limit = 100
	}
	p := Paging{Page: int(page), Limit: int(limit)}
	bps, err := s.port.ListBranchProtections(ctx, r.Owner, r.Repo, p)
	if err != nil {
		return model.BranchProtectionList{}, fmt.Errorf("list branch protections err: %w", err)
	}
	return model.BranchProtectionList{Page: p.Page, Limit: p.Limit, Count: len(bps), BranchProtections: bps}, nil
}

// Get returns one rule.
func (s *Service) Get(ctx context.Context, r RuleRef) (*model.BranchProtection, error) {
	if r.Owner == "" || r.Repo == "" || r.Rule == "" {
		return nil, fmt.Errorf("owner, repo and rule are required")
	}
	bp, err := s.port.GetBranchProtection(ctx, r.Owner, r.Repo, r.Rule)
	if err != nil {
		return nil, fmt.Errorf("get branch protection err: %w", err)
	}
	return bp, nil
}

// Create adds a rule and returns it.
func (s *Service) Create(ctx context.Context, r CreateRequest) (*model.BranchProtection, error) {
	if r.Owner == "" || r.Repo == "" {
		return nil, fmt.Errorf("owner and repo are required")
	}
	if r.BranchName == "" {
		return nil, fmt.Errorf("branch_name is required")
	}
	in := CreateInput{
		BranchName:            r.BranchName,
		RuleName:              r.RuleName,
		EnablePush:            r.EnablePush,
		EnableStatusCheck:     r.EnableStatusCheck,
		RequiredApprovals:     int64(r.RequiredApprovals),
		BlockOnOutdatedBranch: r.BlockOnOutdatedBranch,
		RequireSignedCommits:  r.RequireSignedCommits,
		DismissStaleApprovals: r.DismissStaleApprovals,
	}
	if r.StatusCheckContexts != nil {
		in.StatusCheckContexts = split.CSV(*r.StatusCheckContexts)
	}
	bp, err := s.port.CreateBranchProtection(ctx, r.Owner, r.Repo, in)
	if err != nil {
		return nil, fmt.Errorf("create branch protection err: %w", err)
	}
	return bp, nil
}

// Edit changes the fields of a rule that the request sets, and returns the rule.
func (s *Service) Edit(ctx context.Context, r EditRequest) (*model.BranchProtection, error) {
	if r.Owner == "" || r.Repo == "" || r.Rule == "" {
		return nil, fmt.Errorf("owner, repo and rule are required")
	}
	in := EditInput{
		EnablePush:                  r.EnablePush,
		EnablePushWhitelist:         r.EnablePushWhitelist,
		EnableMergeWhitelist:        r.EnableMergeWhitelist,
		EnableStatusCheck:           r.EnableStatusCheck,
		EnableApprovalsWhitelist:    r.EnableApprovalsWhitelist,
		BlockOnOutdatedBranch:       r.BlockOnOutdatedBranch,
		DismissStaleApprovals:       r.DismissStaleApprovals,
		RequireSignedCommits:        r.RequireSignedCommits,
		PushWhitelistUsernames:      splitPtr(r.PushWhitelistUsernames),
		MergeWhitelistUsernames:     splitPtr(r.MergeWhitelistUsernames),
		StatusCheckContexts:         splitPtr(r.StatusCheckContexts),
		ApprovalsWhitelistUsernames: splitPtr(r.ApprovalsWhitelistUsernames),
	}
	if r.RequiredApprovals != nil {
		v := int64(*r.RequiredApprovals)
		in.RequiredApprovals = &v
	}
	bp, err := s.port.EditBranchProtection(ctx, r.Owner, r.Repo, r.Rule, in)
	if err != nil {
		return nil, fmt.Errorf("edit branch protection err: %w", err)
	}
	return bp, nil
}

// Delete removes a rule.
func (s *Service) Delete(ctx context.Context, r RuleRef) error {
	if r.Owner == "" || r.Repo == "" || r.Rule == "" {
		return fmt.Errorf("owner, repo and rule are required")
	}
	if err := s.port.DeleteBranchProtection(ctx, r.Owner, r.Repo, r.Rule); err != nil {
		return fmt.Errorf("delete branch protection err: %w", err)
	}
	return nil
}

// splitPtr parses an optional comma-separated list, keeping absence (nil) apart
// from an empty list.
func splitPtr(csv *string) *[]string {
	if csv == nil {
		return nil
	}
	v := split.CSV(*csv)
	return &v
}
