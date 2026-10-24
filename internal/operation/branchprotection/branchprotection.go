// SPDX-License-Identifier: GPL-3.0-or-later

// Package branchprotection provides MCP tools and resources to read and manage
// Forgejo/Codeberg branch protection rules
// (/repos/{owner}/{repo}/branch_protections). Motivated by forgejo-mcp-f6h:
// a repo with no protection let Renovate automerge before CI was green.
package branchprotection

import (
	"context"
	"fmt"

	appbp "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/branchprotection"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/branchprotectionadapter"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/log"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/numarg"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/annotate"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/decode"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcpresult"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/params"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/to"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	ListBranchProtectionsToolName  = "list_branch_protections"
	GetBranchProtectionToolName    = "get_branch_protection"
	CreateBranchProtectionToolName = "create_branch_protection"
	EditBranchProtectionToolName   = "edit_branch_protection"
	DeleteBranchProtectionToolName = "delete_branch_protection"
)

var (
	ListBranchProtectionsTool = mcp.NewTool(
		ListBranchProtectionsToolName,
		annotate.Read(),
		mcp.WithDescription("List a repository's branch protection rules (bounded by page/limit). Returns a response envelope {branch_protections, page, limit, count, has_next}. has_next true means a further page may exist; re-issue the call with page incremented to fetch it."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("page", mcp.Required(), mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("limit", mcp.Required(), mcp.Description(params.Limit), mcp.DefaultNumber(100), mcp.Min(1)),
	)

	GetBranchProtectionTool = mcp.NewTool(
		GetBranchProtectionToolName,
		annotate.Read(),
		mcp.WithDescription("Get a single branch protection rule by name"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("rule", mcp.Required(), mcp.Description(params.BPRule)),
	)

	CreateBranchProtectionTool = mcp.NewTool(
		CreateBranchProtectionToolName,
		annotate.Write(),
		mcp.WithDescription("Create a branch protection rule (e.g. require status checks before merge)"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("branch_name", mcp.Required(), mcp.Description(params.Branch)),
		mcp.WithString("rule_name", mcp.Description(params.BPRuleName)),
		mcp.WithBoolean("enable_push", mcp.Description(params.BPEnablePush)),
		mcp.WithBoolean("enable_status_check", mcp.Description(params.BPEnableStatusCheck)),
		mcp.WithString("status_check_contexts", mcp.Description(params.BPStatusCheckContexts)),
		mcp.WithNumber("required_approvals", mcp.Description(params.BPRequiredApprovals), mcp.Min(0)),
		mcp.WithBoolean("block_on_outdated_branch", mcp.Description(params.BPBlockOnOutdatedBranch)),
		mcp.WithBoolean("require_signed_commits", mcp.Description(params.BPRequireSignedCommits)),
		mcp.WithBoolean("dismiss_stale_approvals", mcp.Description(params.BPDismissStaleApprovals)),
	)

	EditBranchProtectionTool = mcp.NewTool(
		EditBranchProtectionToolName,
		annotate.Write(),
		mcp.WithDescription("Edit a branch protection rule. Only fields you pass are changed; omitted fields are left untouched."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("rule", mcp.Required(), mcp.Description(params.BPRule)),
		mcp.WithBoolean("enable_push", mcp.Description(params.BPEnablePush)),
		mcp.WithBoolean("enable_status_check", mcp.Description(params.BPEnableStatusCheck)),
		mcp.WithString("status_check_contexts", mcp.Description(params.BPStatusCheckContexts)),
		mcp.WithNumber("required_approvals", mcp.Description(params.BPRequiredApprovals), mcp.Min(0)),
		mcp.WithBoolean("block_on_outdated_branch", mcp.Description(params.BPBlockOnOutdatedBranch)),
		mcp.WithBoolean("require_signed_commits", mcp.Description(params.BPRequireSignedCommits)),
		mcp.WithBoolean("dismiss_stale_approvals", mcp.Description(params.BPDismissStaleApprovals)),
		mcp.WithBoolean("enable_push_whitelist", mcp.Description(params.BPEnablePushWhitelist)),
		mcp.WithString("push_whitelist_usernames", mcp.Description(params.BPPushWhitelistUsers)),
		mcp.WithBoolean("enable_merge_whitelist", mcp.Description(params.BPEnableMergeWhitelist)),
		mcp.WithString("merge_whitelist_usernames", mcp.Description(params.BPMergeWhitelistUsers)),
		mcp.WithBoolean("enable_approvals_whitelist", mcp.Description(params.BPEnableApprovalsWl)),
		mcp.WithString("approvals_whitelist_usernames", mcp.Description(params.BPApprovalsWhitelistUsers)),
	)

	DeleteBranchProtectionTool = mcp.NewTool(
		DeleteBranchProtectionToolName,
		annotate.Destructive(),
		mcp.WithDescription("Delete a branch protection rule by name"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("rule", mcp.Required(), mcp.Description(params.BPRule)),
	)
)

// RegisterTool registers the branch protection CRUD tools on the MCP server.
func RegisterTool(s *server.MCPServer, f forge.Factory) {
	h := handlers{forge: f}
	s.AddTool(ListBranchProtectionsTool, h.ListBranchProtectionsFn)
	s.AddTool(GetBranchProtectionTool, h.GetBranchProtectionFn)
	s.AddTool(CreateBranchProtectionTool, h.CreateBranchProtectionFn)
	s.AddTool(EditBranchProtectionTool, h.EditBranchProtectionFn)
	s.AddTool(DeleteBranchProtectionTool, h.DeleteBranchProtectionFn)
}

// handlers serve the branch protection tools and resources. They hold the
// factory rather than a port so each call is bound to the credential of its
// own request. Each tool handler is the MCP adapter of one use case in
// internal/app/branchprotection: decode the arguments, run the use case, render the
// result.
type handlers struct{ forge forge.Factory }

// Argument structs, one per tool. `json` names the MCP argument and
// `required:"true"` marks the required ones; mcptest.ArgsMatchSchema keeps
// them in agreement with the tool definitions. Optional fields that must tell
// "not passed" from a zero value are pointers.

type listArgs struct {
	Owner string  `json:"owner" required:"true"`
	Repo  string  `json:"repo" required:"true"`
	Page  float64 `json:"page" required:"true"`
	Limit float64 `json:"limit" required:"true"`
}

type ruleArgs struct {
	Owner string `json:"owner" required:"true"`
	Repo  string `json:"repo" required:"true"`
	Rule  string `json:"rule" required:"true"`
}

type createArgs struct {
	Owner                 string  `json:"owner" required:"true"`
	Repo                  string  `json:"repo" required:"true"`
	BranchName            string  `json:"branch_name" required:"true"`
	RuleName              string  `json:"rule_name"`
	EnablePush            bool    `json:"enable_push"`
	EnableStatusCheck     bool    `json:"enable_status_check"`
	StatusCheckContexts   *string `json:"status_check_contexts"`
	RequiredApprovals     float64 `json:"required_approvals"`
	BlockOnOutdatedBranch bool    `json:"block_on_outdated_branch"`
	RequireSignedCommits  bool    `json:"require_signed_commits"`
	DismissStaleApprovals bool    `json:"dismiss_stale_approvals"`
}

type editArgs struct {
	Owner                       string   `json:"owner" required:"true"`
	Repo                        string   `json:"repo" required:"true"`
	Rule                        string   `json:"rule" required:"true"`
	EnablePush                  *bool    `json:"enable_push"`
	EnablePushWhitelist         *bool    `json:"enable_push_whitelist"`
	PushWhitelistUsernames      *string  `json:"push_whitelist_usernames"`
	EnableMergeWhitelist        *bool    `json:"enable_merge_whitelist"`
	MergeWhitelistUsernames     *string  `json:"merge_whitelist_usernames"`
	EnableStatusCheck           *bool    `json:"enable_status_check"`
	StatusCheckContexts         *string  `json:"status_check_contexts"`
	RequiredApprovals           *float64 `json:"required_approvals" numeric:"string"`
	EnableApprovalsWhitelist    *bool    `json:"enable_approvals_whitelist"`
	ApprovalsWhitelistUsernames *string  `json:"approvals_whitelist_usernames"`
	BlockOnOutdatedBranch       *bool    `json:"block_on_outdated_branch"`
	DismissStaleApprovals       *bool    `json:"dismiss_stale_approvals"`
	RequireSignedCommits        *bool    `json:"require_signed_commits"`
}

// run is the shared shape of a tool call: decode the arguments into A, take the
// use cases bound to this request's credential, and hand both to do, which
// runs one use case and renders its result.
func run[A any](ctx context.Context, h handlers, req mcp.CallToolRequest, name string,
	do func(context.Context, *appbp.Service, A) (*mcp.CallToolResult, error),
) (*mcp.CallToolResult, error) {
	log.Debugf("Called %s", name)
	var args A
	if err := decode.Lax(req, &args); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	port, err := h.forge.For(ctx)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return do(ctx, appbp.New(branchprotectionadapter.New(port)), args)
}

func (h handlers) ListBranchProtectionsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "ListBranchProtectionsFn", func(ctx context.Context, s *appbp.Service, a listArgs) (*mcp.CallToolResult, error) {
		pg, lim, perr := decode.PagingArgs(a.Page, a.Limit)
		if perr != nil {
			return mcpresult.Fail(perr)
		}
		got, err := s.List(ctx, appbp.ListRequest{Owner: a.Owner, Repo: a.Repo, Page: pg, Limit: lim})
		if err != nil {
			return mcpresult.Fail(err)
		}
		return mcpresult.Render(model.BranchProtectionList{BranchProtections: got.Items, PageMeta: model.PageMetaOf(got)}, nil)
	})
}

func (h handlers) GetBranchProtectionFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "GetBranchProtectionFn", func(ctx context.Context, s *appbp.Service, a ruleArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render(s.Get(ctx, appbp.RuleRef{Owner: a.Owner, Repo: a.Repo, Rule: a.Rule}))
	})
}

func (h handlers) CreateBranchProtectionFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "CreateBranchProtectionFn", func(ctx context.Context, s *appbp.Service, a createArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render(s.Create(ctx, appbp.CreateRequest{
			Owner: a.Owner, Repo: a.Repo, BranchName: a.BranchName, RuleName: a.RuleName,
			EnablePush: a.EnablePush, EnableStatusCheck: a.EnableStatusCheck, StatusCheckContexts: a.StatusCheckContexts,
			RequiredApprovals: numarg.Truncate(a.RequiredApprovals), BlockOnOutdatedBranch: a.BlockOnOutdatedBranch,
			RequireSignedCommits: a.RequireSignedCommits, DismissStaleApprovals: a.DismissStaleApprovals,
		}))
	})
}

func (h handlers) EditBranchProtectionFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "EditBranchProtectionFn", func(ctx context.Context, s *appbp.Service, a editArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render(s.Edit(ctx, appbp.EditRequest{
			RuleRef:    appbp.RuleRef{Owner: a.Owner, Repo: a.Repo, Rule: a.Rule},
			EnablePush: a.EnablePush, EnablePushWhitelist: a.EnablePushWhitelist, PushWhitelistUsernames: a.PushWhitelistUsernames,
			EnableMergeWhitelist: a.EnableMergeWhitelist, MergeWhitelistUsernames: a.MergeWhitelistUsernames,
			EnableStatusCheck: a.EnableStatusCheck, StatusCheckContexts: a.StatusCheckContexts, RequiredApprovals: optionalApprovals(a.RequiredApprovals),
			EnableApprovalsWhitelist: a.EnableApprovalsWhitelist, ApprovalsWhitelistUsernames: a.ApprovalsWhitelistUsernames,
			BlockOnOutdatedBranch: a.BlockOnOutdatedBranch, DismissStaleApprovals: a.DismissStaleApprovals,
			RequireSignedCommits: a.RequireSignedCommits,
		}))
	})
}

func (h handlers) DeleteBranchProtectionFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "DeleteBranchProtectionFn", func(ctx context.Context, s *appbp.Service, a ruleArgs) (*mcp.CallToolResult, error) {
		if err := s.Delete(ctx, appbp.RuleRef{Owner: a.Owner, Repo: a.Repo, Rule: a.Rule}); err != nil {
			return mcpresult.Fail(err)
		}
		return to.TextResult(fmt.Sprintf("Deleted branch protection rule %q on %s/%s", a.Rule, a.Owner, a.Repo))
	})
}

// optionalApprovals truncates an optional required_approvals to the whole number
// the use case takes. A nil argument stays nil (not passed).
func optionalApprovals(v *float64) *int64 {
	if v == nil {
		return nil
	}
	n := numarg.Truncate(*v)
	return &n
}
