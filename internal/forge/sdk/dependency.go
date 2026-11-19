// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"fmt"
	"net/http"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var _ forge.Dependencies = (*Adapter)(nil)

func pageQuery(page, limit int) string { return fmt.Sprintf("page=%d&limit=%d", page, limit) }

// issueMetaBody is the IssueMeta body Forgejo's dependency mutation endpoints
// take: owner, repo and index of the other issue.
type issueMetaBody struct {
	Index int64  `json:"index"`
	Owner string `json:"owner"`
	Repo  string `json:"repo"`
}

// listIssues lists issues at an issue sub-resource. A 404 is an empty list only
// when the parent issue exists; a missing parent is not found (see notFoundOr).
func (a *Adapter) listIssues(ctx context.Context, owner, repo string, index int64, sub string, page, limit int) ([]*forgejo_sdk.Issue, error) {
	base, err := forge.APIPath("repos", owner, repo, "issues", index, sub)
	if err != nil {
		return nil, err
	}
	path := base + "?" + pageQuery(page, limit)
	issues := []*forgejo_sdk.Issue{}
	_, err = a.rawJSON(ctx, http.MethodGet, path, nil, &issues)
	if forge.IsNotFound(err) {
		_, probe := a.GetIssue(ctx, owner, repo, index)
		if err := notFoundOr(probe, fmt.Sprintf("issue %d in %s/%s not found", index, owner, repo)); err != nil {
			return nil, err
		}
		return []*forgejo_sdk.Issue{}, nil
	}
	if err != nil {
		return nil, err
	}
	return issues, nil
}

func (a *Adapter) ListIssueDependencies(ctx context.Context, owner, repo string, index int64, page, limit int) ([]*forgejo_sdk.Issue, error) {
	return a.listIssues(ctx, owner, repo, index, "dependencies", page, limit)
}

func (a *Adapter) ListIssueDependents(ctx context.Context, owner, repo string, index int64, page, limit int) ([]*forgejo_sdk.Issue, error) {
	return a.listIssues(ctx, owner, repo, index, "blocks", page, limit)
}

func (a *Adapter) AddIssueDependency(ctx context.Context, owner, repo string, index int64, dep forge.IssueRef) error {
	path, err := forge.APIPath("repos", owner, repo, "issues", index, "dependencies")
	if err != nil {
		return err
	}
	_, err = a.rawJSON(ctx, http.MethodPost, path, issueMetaBody{Index: dep.Index, Owner: dep.Owner, Repo: dep.Repo}, nil)
	return err
}

func (a *Adapter) RemoveIssueDependency(ctx context.Context, owner, repo string, index int64, dep forge.IssueRef) error {
	path, err := forge.APIPath("repos", owner, repo, "issues", index, "dependencies")
	if err != nil {
		return err
	}
	_, err = a.rawJSON(ctx, http.MethodDelete, path, issueMetaBody{Index: dep.Index, Owner: dep.Owner, Repo: dep.Repo}, nil)
	return err
}
