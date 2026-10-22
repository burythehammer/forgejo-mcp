// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"net/http"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var _ forge.Labels = (*Adapter)(nil)

func (a *Adapter) ListRepoLabels(_ context.Context, owner, repo string, opt forgejo_sdk.ListLabelsOptions) ([]*forgejo_sdk.Label, forge.PageInfo, error) {
	labels, resp, err := a.client.ListRepoLabels(owner, repo, opt)
	return labels, pageInfo(resp), statusErr(resp, err)
}

func (a *Adapter) GetRepoLabel(_ context.Context, owner, repo string, id int64) (*forgejo_sdk.Label, error) {
	l, resp, err := a.client.GetRepoLabel(owner, repo, id)
	return l, statusErr(resp, err)
}

func (a *Adapter) CreateRepoLabel(_ context.Context, owner, repo string, opt forgejo_sdk.CreateLabelOption) (*forgejo_sdk.Label, error) {
	l, resp, err := a.client.CreateLabel(owner, repo, opt)
	return l, statusErr(resp, err)
}

func (a *Adapter) EditRepoLabel(_ context.Context, owner, repo string, id int64, opt forgejo_sdk.EditLabelOption) (*forgejo_sdk.Label, error) {
	l, resp, err := a.client.EditLabel(owner, repo, id, opt)
	return l, statusErr(resp, err)
}

func (a *Adapter) DeleteRepoLabel(_ context.Context, owner, repo string, id int64) error {
	resp, err := a.client.DeleteLabel(owner, repo, id)
	return statusErr(resp, err)
}

// The SDK has no organization-label endpoints, so these go around it. The
// bodies omit empty fields: an org label is created without a description
// unless one is given, and edited only in the fields supplied.

type orgLabelCreate struct {
	Name        string  `json:"name,omitempty"`
	Color       string  `json:"color,omitempty"`
	Description *string `json:"description,omitempty"`
}

type orgLabelPatch struct {
	Name        *string `json:"name,omitempty"`
	Color       *string `json:"color,omitempty"`
	Description *string `json:"description,omitempty"`
}

func (a *Adapter) ListOrgLabels(ctx context.Context, org string, page, limit int) ([]*forgejo_sdk.Label, forge.PageInfo, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 100
	}
	path := forge.APIPath("orgs", org, "labels") + "?" + pageQuery(page, limit)
	labels := []*forgejo_sdk.Label{}
	header, err := a.rawJSON(ctx, http.MethodGet, path, nil, &labels)
	if forge.IsNotFound(err) {
		// Forgejo answers 404 for a missing org too; the org decides.
		_, probe := a.GetOrg(ctx, org)
		if perr := notFoundOr(probe, orgNotFound(org)); perr != nil {
			return nil, forge.PageInfo{}, perr
		}
		return []*forgejo_sdk.Label{}, forge.PageInfo{}, nil
	}
	if err != nil {
		return nil, forge.PageInfo{}, err
	}
	return labels, headerPageInfo(header), nil
}

func (a *Adapter) GetOrgLabel(ctx context.Context, org string, id int64) (*forgejo_sdk.Label, error) {
	var l forgejo_sdk.Label
	if _, err := a.rawJSON(ctx, http.MethodGet, forge.APIPath("orgs", org, "labels", id), nil, &l); err != nil {
		return nil, err
	}
	return &l, nil
}

func (a *Adapter) CreateOrgLabel(ctx context.Context, org string, opt forgejo_sdk.CreateLabelOption) (*forgejo_sdk.Label, error) {
	body := orgLabelCreate{Name: opt.Name, Color: opt.Color}
	if opt.Description != "" {
		body.Description = &opt.Description
	}
	var l forgejo_sdk.Label
	if _, err := a.rawJSON(ctx, http.MethodPost, forge.APIPath("orgs", org, "labels"), body, &l); err != nil {
		return nil, err
	}
	return &l, nil
}

func (a *Adapter) EditOrgLabel(ctx context.Context, org string, id int64, opt forgejo_sdk.EditLabelOption) (*forgejo_sdk.Label, error) {
	body := orgLabelPatch{Name: opt.Name, Color: opt.Color, Description: opt.Description}
	var l forgejo_sdk.Label
	if _, err := a.rawJSON(ctx, http.MethodPatch, forge.APIPath("orgs", org, "labels", id), body, &l); err != nil {
		return nil, err
	}
	return &l, nil
}

func (a *Adapter) DeleteOrgLabel(ctx context.Context, org string, id int64) error {
	_, err := a.rawJSON(ctx, http.MethodDelete, forge.APIPath("orgs", org, "labels", id), nil, nil)
	return err
}

func (a *Adapter) ListOrgRepoNames(_ context.Context, org string, page, limit int) ([]string, error) {
	repos, resp, err := a.client.ListOrgRepos(org, forgejo_sdk.ListOrgReposOptions{
		ListOptions: forgejo_sdk.ListOptions{Page: page, PageSize: limit},
	})
	if err = statusErr(resp, err); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(repos))
	for _, r := range repos {
		names = append(names, r.Name)
	}
	return names, nil
}

func (a *Adapter) AddIssueLabels(_ context.Context, owner, repo string, index int64, labelIDs []int64) error {
	_, resp, err := a.client.AddIssueLabels(owner, repo, index, forgejo_sdk.IssueLabelsOption{Labels: labelIDs})
	return statusErr(resp, err)
}

func (a *Adapter) RemoveIssueLabel(_ context.Context, owner, repo string, index, labelID int64) error {
	resp, err := a.client.DeleteIssueLabel(owner, repo, index, labelID)
	return statusErr(resp, err)
}
