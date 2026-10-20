// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var _ forge.Issues = (*Adapter)(nil)

// pageInfo reads the SDK's parsed Link header and X-Total-Count.
func pageInfo(resp *forgejo_sdk.Response) forge.PageInfo {
	if resp == nil || resp.Response == nil {
		return forge.PageInfo{}
	}
	info := headerPageInfo(resp.Header)
	info.HasNext = resp.NextPage != 0
	return info
}

// headerPageInfo is pageInfo for the raw-HTTP path, which has no parsed Response.
func headerPageInfo(h http.Header) forge.PageInfo {
	var info forge.PageInfo
	for _, link := range h.Values("Link") {
		if strings.Contains(link, `rel="next"`) || strings.Contains(link, "rel=next") {
			info.HasNext = true
		}
	}
	info.TotalCount, info.TotalKnown = totalCountOK(h)
	return info
}

func (a *Adapter) GetIssue(_ context.Context, owner, repo string, index int64) (*forgejo_sdk.Issue, error) {
	iss, resp, err := a.client.GetIssue(owner, repo, index)
	return iss, statusErr(resp, err)
}

func (a *Adapter) ListRepoIssues(ctx context.Context, owner, repo string, opt forge.ListIssuesOptions) ([]*forgejo_sdk.Issue, forge.PageInfo, error) {
	// The Forgejo API supports `sort` on this endpoint (nearduedate and
	// farduedate included) but the SDK's ListIssueOption has no Sort field to
	// carry it (v3.0.0 QueryEncode never emits &sort=...), so a sorted listing
	// goes around the SDK.
	//
	// DoJSON, not DoJSONList: DoJSONList maps a 404 to an empty list, but this
	// endpoint answers 200 [] for a repo with no issues, so a 404 means the
	// repository does not exist and must not read as "no issues".
	if opt.Sort != "" {
		query := opt.QueryEncode() + "&sort=" + url.QueryEscape(opt.Sort)
		path := forge.APIPath("repos", owner, repo, "issues") + "?" + query
		var issues []*forgejo_sdk.Issue
		header, err := a.rawJSON(ctx, http.MethodGet, path, nil, &issues)
		if err != nil {
			return nil, forge.PageInfo{}, err
		}
		return issues, headerPageInfo(header), nil
	}
	issues, resp, err := a.client.ListRepoIssues(owner, repo, opt.ListIssueOption)
	return issues, pageInfo(resp), statusErr(resp, err)
}

func (a *Adapter) SearchIssues(_ context.Context, opt forgejo_sdk.ListIssueOption) ([]*forgejo_sdk.Issue, forge.PageInfo, error) {
	issues, resp, err := a.client.ListIssues(opt)
	return issues, pageInfo(resp), statusErr(resp, err)
}

func (a *Adapter) CreateIssue(_ context.Context, owner, repo string, opt forgejo_sdk.CreateIssueOption) (*forgejo_sdk.Issue, error) {
	// The SDK refuses an empty title before it sends anything, so no status
	// comes back. Forgejo answers that refusal with 422; say the same here so
	// the adapter reports the same validation kind as the fake and the server.
	if strings.TrimSpace(opt.Title) == "" {
		return nil, &forge.StatusError{StatusCode: http.StatusUnprocessableEntity, Err: errors.New("title is empty")}
	}
	iss, resp, err := a.client.CreateIssue(owner, repo, opt)
	return iss, statusErr(resp, err)
}

func (a *Adapter) EditIssue(_ context.Context, owner, repo string, index int64, opt forgejo_sdk.EditIssueOption) (*forgejo_sdk.Issue, error) {
	iss, resp, err := a.client.EditIssue(owner, repo, index, opt)
	return iss, statusErr(resp, err)
}

func (a *Adapter) ReplaceIssueLabels(_ context.Context, owner, repo string, index int64, labelIDs []int64) error {
	_, resp, err := a.client.ReplaceIssueLabels(owner, repo, index, forgejo_sdk.IssueLabelsOption{Labels: labelIDs})
	return statusErr(resp, err)
}

func (a *Adapter) ClearIssueLabels(_ context.Context, owner, repo string, index int64) error {
	resp, err := a.client.ClearIssueLabels(owner, repo, index)
	return statusErr(resp, err)
}

func (a *Adapter) ListIssueComments(_ context.Context, owner, repo string, index int64, opt forgejo_sdk.ListIssueCommentOptions) ([]*forgejo_sdk.Comment, forge.PageInfo, error) {
	comments, resp, err := a.client.ListIssueComments(owner, repo, index, opt)
	return comments, pageInfo(resp), statusErr(resp, err)
}

func (a *Adapter) GetIssueComment(_ context.Context, owner, repo string, id int64) (*forgejo_sdk.Comment, error) {
	c, resp, err := a.client.GetIssueComment(owner, repo, id)
	return c, statusErr(resp, err)
}

func (a *Adapter) CreateIssueComment(_ context.Context, owner, repo string, index int64, body string) (*forgejo_sdk.Comment, error) {
	c, resp, err := a.client.CreateIssueComment(owner, repo, index, forgejo_sdk.CreateIssueCommentOption{Body: body})
	return c, statusErr(resp, err)
}

func (a *Adapter) EditIssueComment(_ context.Context, owner, repo string, id int64, body string) (*forgejo_sdk.Comment, error) {
	c, resp, err := a.client.EditIssueComment(owner, repo, id, forgejo_sdk.EditIssueCommentOption{Body: body})
	return c, statusErr(resp, err)
}

func (a *Adapter) DeleteIssueComment(_ context.Context, owner, repo string, id int64) error {
	resp, err := a.client.DeleteIssueComment(owner, repo, id)
	return statusErr(resp, err)
}

func (a *Adapter) ListRepoMilestones(_ context.Context, owner, repo string, opt forgejo_sdk.ListMilestoneOption) ([]*forgejo_sdk.Milestone, error) {
	ms, resp, err := a.client.ListRepoMilestones(owner, repo, opt)
	return ms, statusErr(resp, err)
}
