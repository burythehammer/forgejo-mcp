// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"fmt"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/to"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

// The label-assignment and label-listing tools.

// ScopedLabel wraps forgejo_sdk.Label with a scope marker so callers of
// list_repo_labels and list_org_labels can tell repo- and org-scoped
// labels apart in a merged response.
type ScopedLabel struct {
	*forgejo_sdk.Label
	Scope string `json:"scope"`
}

// listOrgLabels pages an organization's labels and stamps each with
// scope="org". An owner that is not an organization has none (the port maps
// Forgejo's 404 to an empty list); 401/403 surface as errors.
//
// The page info comes back alongside the labels because the bounded org-labels
// resource pages: it requests exactly the caller's limit and reads it to decide
// whether more rows exist. Tool callers that enumerate without bounds can
// discard it.
func listOrgLabels(ctx context.Context, p forge.Labels, org string, page, limit int) ([]ScopedLabel, forge.PageInfo, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 100
	}
	raw, info, err := p.ListOrgLabels(ctx, org, page, limit)
	if err != nil {
		return nil, info, err
	}
	out := make([]ScopedLabel, 0, len(raw))
	for _, l := range raw {
		out = append(out, ScopedLabel{Label: l, Scope: "org"})
	}
	return out, info, nil
}

func (h handlers) AddIssueLabelsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called AddIssueLabelsFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, _ := to.Float64(req.GetArguments()["index"])
	labels, _ := req.GetArguments()["labels"].(string)

	labelIDs, err := h.resolveLabelIDs(ctx, owner, repo, labels)
	if err != nil {
		return to.ErrorResult(err)
	}

	fg, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	if err := fg.AddIssueLabels(ctx, owner, repo, int64(index), labelIDs); err != nil {
		return to.ErrorResult(fmt.Errorf("add issue labels err: %w", err))
	}

	// Fetch the updated issue to return it with the new labels
	issue, err := fg.GetIssue(ctx, owner, repo, int64(index))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get updated issue err: %w", err))
	}
	return to.TextResult(issue)
}

func (h handlers) RemoveIssueLabelsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called RemoveIssueLabelsFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, _ := to.Float64(req.GetArguments()["index"])
	labels, _ := req.GetArguments()["labels"].(string)

	labelIDs, err := h.resolveLabelIDs(ctx, owner, repo, labels)
	if err != nil {
		return to.ErrorResult(err)
	}

	fg, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}

	for _, labelID := range labelIDs {
		if derr := fg.RemoveIssueLabel(ctx, owner, repo, int64(index), labelID); derr != nil {
			return to.ErrorResult(fmt.Errorf("remove issue label err: %w", derr))
		}
	}

	// Fetch the updated issue to return it with the updated labels
	issue, err := fg.GetIssue(ctx, owner, repo, int64(index))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get updated issue err: %w", err))
	}
	return to.TextResult(issue)
}

func (h handlers) ListRepoLabelsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListRepoLabelsFn")
	args := req.GetArguments()
	owner, _ := args["owner"].(string)
	repo, _ := args["repo"].(string)
	page, _ := to.Float64(args["page"])
	if page == 0 {
		page = 1
	}
	limit, _ := to.Float64(args["limit"])
	if limit == 0 {
		limit = 100
	}
	includeOrg := true
	if v, ok := args["include_org_labels"].(bool); ok {
		includeOrg = v
	}

	opt := forgejo_sdk.ListLabelsOptions{
		ListOptions: forgejo_sdk.ListOptions{
			Page:     int(page),
			PageSize: int(limit),
		},
	}

	fg, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	repoLabels, _, err := fg.ListRepoLabels(ctx, owner, repo, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list repo labels err: %w", err))
	}
	merged := make([]ScopedLabel, 0, len(repoLabels))
	for _, l := range repoLabels {
		merged = append(merged, ScopedLabel{Label: l, Scope: "repo"})
	}

	if includeOrg {
		orgLabels, _, oerr := listOrgLabels(ctx, fg, owner, int(page), int(limit))
		if oerr != nil {
			return to.ErrorResult(fmt.Errorf("list org labels err: %w", oerr))
		}
		merged = append(merged, orgLabels...)
	}

	return to.TextResult(merged)
}

func (h handlers) ListOrgLabelsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListOrgLabelsFn")
	args := req.GetArguments()
	org, _ := args["org"].(string)
	page, _ := to.Float64(args["page"])
	if page == 0 {
		page = 1
	}
	limit, _ := to.Float64(args["limit"])
	if limit == 0 {
		limit = 100
	}

	fg, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	labels, _, err := listOrgLabels(ctx, fg, org, int(page), int(limit))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list org labels err: %w", err))
	}
	return to.TextResult(labels)
}
