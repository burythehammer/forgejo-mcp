// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var _ forge.Labels = (*Adapter)(nil)

// The SDK's label types predate the exclusive flag, so every label call goes
// around it: the bodies and replies are forge.Label, LabelCreate and
// LabelPatch, whose JSON is Forgejo's. Only repository label deletion and the
// issue label calls still use the SDK.

func (a *Adapter) ListRepoLabels(ctx context.Context, owner, repo string, page, limit int) ([]*forge.Label, forge.PageInfo, error) {
	base, err := forge.APIPath("repos", owner, repo, "labels")
	if err != nil {
		return nil, forge.PageInfo{}, err
	}
	labels := []*forge.Label{}
	header, err := a.rawJSON(ctx, http.MethodGet, base+"?"+labelPageQuery(page, limit), nil, &labels)
	if err != nil {
		return nil, forge.PageInfo{}, err
	}
	return labels, headerPageInfo(header), nil
}

func (a *Adapter) GetRepoLabel(ctx context.Context, owner, repo string, id int64) (*forge.Label, error) {
	path, err := forge.APIPath("repos", owner, repo, "labels", id)
	if err != nil {
		return nil, err
	}
	var l forge.Label
	if _, err := a.rawJSON(ctx, http.MethodGet, path, nil, &l); err != nil {
		return nil, err
	}
	return &l, nil
}

func (a *Adapter) CreateRepoLabel(ctx context.Context, owner, repo string, opt forge.LabelCreate) (*forge.Label, error) {
	path, err := forge.APIPath("repos", owner, repo, "labels")
	if err != nil {
		return nil, err
	}
	var l forge.Label
	if _, err := a.rawJSON(ctx, http.MethodPost, path, opt, &l); err != nil {
		return nil, err
	}
	if err := honoursCreate(opt, &l); err != nil {
		return nil, err
	}
	return &l, nil
}

func (a *Adapter) EditRepoLabel(ctx context.Context, owner, repo string, id int64, opt forge.LabelPatch) (*forge.Label, error) {
	path, err := forge.APIPath("repos", owner, repo, "labels", id)
	if err != nil {
		return nil, err
	}
	var l forge.Label
	if _, err := a.rawJSON(ctx, http.MethodPatch, path, opt, &l); err != nil {
		return nil, err
	}
	if err := honoursPatch(opt, &l); err != nil {
		return nil, err
	}
	return &l, nil
}

func (a *Adapter) DeleteRepoLabel(_ context.Context, owner, repo string, id int64) error {
	resp, err := a.client.DeleteLabel(owner, repo, id)
	return statusErr(resp, err)
}

// honoursCreate fails a create whose stored exclusive flag is not the one
// asked for. Forgejo that ignores the field must not look like success.
func honoursCreate(opt forge.LabelCreate, got *forge.Label) error {
	if got.Exclusive == opt.Exclusive {
		return nil
	}
	return fmt.Errorf("forgejo did not store exclusive=%t on label %q", opt.Exclusive, got.Name)
}

// honoursPatch is honoursCreate for an edit: an edit that omits Exclusive has
// nothing to check.
func honoursPatch(opt forge.LabelPatch, got *forge.Label) error {
	if opt.Exclusive == nil || got.Exclusive == *opt.Exclusive {
		return nil
	}
	return fmt.Errorf("forgejo did not store exclusive=%t on label %d", *opt.Exclusive, got.ID)
}

// labelPageQuery is the page query for a label list. An unset limit is left to
// the server's default.
func labelPageQuery(page, limit int) string {
	if page < 1 {
		page = 1
	}
	q := url.Values{}
	q.Set("page", fmt.Sprint(page))
	if limit > 0 {
		q.Set("limit", fmt.Sprint(limit))
	}
	return q.Encode()
}

func (a *Adapter) ListOrgLabels(ctx context.Context, org string, page, limit int) ([]*forge.Label, forge.PageInfo, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 100
	}
	base, err := forge.APIPath("orgs", org, "labels")
	if err != nil {
		return nil, forge.PageInfo{}, err
	}
	path := base + "?" + pageQuery(page, limit)
	labels := []*forge.Label{}
	header, err := a.rawJSON(ctx, http.MethodGet, path, nil, &labels)
	if forge.IsNotFound(err) {
		// Forgejo answers 404 for a missing org too; the org decides.
		_, probe := a.GetOrg(ctx, org)
		if perr := notFoundOr(probe, orgNotFound(org)); perr != nil {
			return nil, forge.PageInfo{}, perr
		}
		return []*forge.Label{}, forge.PageInfo{}, nil
	}
	if err != nil {
		return nil, forge.PageInfo{}, err
	}
	return labels, headerPageInfo(header), nil
}

func (a *Adapter) GetOrgLabel(ctx context.Context, org string, id int64) (*forge.Label, error) {
	var l forge.Label
	path, err := forge.APIPath("orgs", org, "labels", id)
	if err != nil {
		return nil, err
	}
	if _, err := a.rawJSON(ctx, http.MethodGet, path, nil, &l); err != nil {
		return nil, err
	}
	return &l, nil
}

func (a *Adapter) CreateOrgLabel(ctx context.Context, org string, opt forge.LabelCreate) (*forge.Label, error) {
	var l forge.Label
	path, err := forge.APIPath("orgs", org, "labels")
	if err != nil {
		return nil, err
	}
	if _, err := a.rawJSON(ctx, http.MethodPost, path, opt, &l); err != nil {
		return nil, err
	}
	if err := honoursCreate(opt, &l); err != nil {
		return nil, err
	}
	return &l, nil
}

func (a *Adapter) EditOrgLabel(ctx context.Context, org string, id int64, opt forge.LabelPatch) (*forge.Label, error) {
	var l forge.Label
	path, err := forge.APIPath("orgs", org, "labels", id)
	if err != nil {
		return nil, err
	}
	if _, err := a.rawJSON(ctx, http.MethodPatch, path, opt, &l); err != nil {
		return nil, err
	}
	if err := honoursPatch(opt, &l); err != nil {
		return nil, err
	}
	return &l, nil
}

func (a *Adapter) DeleteOrgLabel(ctx context.Context, org string, id int64) error {
	path, err := forge.APIPath("orgs", org, "labels", id)
	if err != nil {
		return err
	}
	_, err = a.rawJSON(ctx, http.MethodDelete, path, nil, nil)
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
