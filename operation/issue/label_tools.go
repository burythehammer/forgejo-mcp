// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"fmt"
	"net/http"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/to"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

// The label-assignment and label-listing tools. They still use the
// forgejo.Client(ctx) singleton; the labels ticket migrates them.

// ScopedLabel wraps forgejo_sdk.Label with a scope marker so callers of
// list_repo_labels and list_org_labels can tell repo- and org-scoped
// labels apart in a merged response.
type ScopedLabel struct {
	*forgejo_sdk.Label
	Scope string `json:"scope"`
}

// fetchOrgLabels GETs /orgs/{org}/labels via the raw-HTTP helper and
// stamps each result with scope="org". A 404 is mapped to an empty slice
// by DoJSONListWithHeader. 401/403 surface as forgejo.ErrUnauthorized.
//
// The response headers come back alongside the labels because the bounded
// org-labels resource pages: it requests exactly the caller's limit and reads
// Link/X-Total-Count to decide whether more rows exist. Tool callers that
// enumerate without bounds can discard them.
func fetchOrgLabels(ctx context.Context, org string, page, limit int) ([]ScopedLabel, http.Header, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 100
	}
	path := forgejo.APIPath("orgs", org, "labels") + fmt.Sprintf("?page=%d&limit=%d", page, limit)
	var raw []*forgejo_sdk.Label
	header, err := forgejo.DoJSONListWithHeader(ctx, http.MethodGet, path, &raw)
	if err != nil {
		return nil, header, err
	}
	out := make([]ScopedLabel, 0, len(raw))
	for _, l := range raw {
		out = append(out, ScopedLabel{Label: l, Scope: "org"})
	}
	return out, header, nil
}

func AddIssueLabelsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called AddIssueLabelsFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, _ := to.Float64(req.GetArguments()["index"])
	labels, _ := req.GetArguments()["labels"].(string)

	labelIDs, err := resolveIssueLabelIDs(ctx, owner, repo, labels)
	if err != nil {
		return to.ErrorResult(err)
	}

	opt := forgejo_sdk.IssueLabelsOption{
		Labels: labelIDs,
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	_, _, err = client.AddIssueLabels(owner, repo, int64(index), opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("add issue labels err: %w", err))
	}

	// Fetch the updated issue to return it with the new labels
	issue, _, err := client.GetIssue(owner, repo, int64(index))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get updated issue err: %w", err))
	}
	return to.TextResult(issue)
}

func RemoveIssueLabelsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called RemoveIssueLabelsFn")
	owner, _ := req.GetArguments()["owner"].(string)
	repo, _ := req.GetArguments()["repo"].(string)
	index, _ := to.Float64(req.GetArguments()["index"])
	labels, _ := req.GetArguments()["labels"].(string)

	labelIDs, err := resolveIssueLabelIDs(ctx, owner, repo, labels)
	if err != nil {
		return to.ErrorResult(err)
	}

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}

	for _, labelID := range labelIDs {
		if _, derr := client.DeleteIssueLabel(owner, repo, int64(index), labelID); derr != nil {
			return to.ErrorResult(fmt.Errorf("remove issue label err: %w", derr))
		}
	}

	// Fetch the updated issue to return it with the updated labels
	issue, _, err := client.GetIssue(owner, repo, int64(index))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get updated issue err: %w", err))
	}
	return to.TextResult(issue)
}

func ListRepoLabelsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

	client, err := forgejo.Client(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	repoLabels, _, err := client.ListRepoLabels(owner, repo, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list repo labels err: %w", err))
	}
	merged := make([]ScopedLabel, 0, len(repoLabels))
	for _, l := range repoLabels {
		merged = append(merged, ScopedLabel{Label: l, Scope: "repo"})
	}

	if includeOrg {
		orgLabels, _, oerr := fetchOrgLabels(ctx, owner, int(page), int(limit))
		if oerr != nil {
			return to.ErrorResult(fmt.Errorf("list org labels err: %w", oerr))
		}
		merged = append(merged, orgLabels...)
	}

	return to.TextResult(merged)
}

func ListOrgLabelsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

	labels, _, err := fetchOrgLabels(ctx, org, int(page), int(limit))
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list org labels err: %w", err))
	}
	return to.TextResult(labels)
}
