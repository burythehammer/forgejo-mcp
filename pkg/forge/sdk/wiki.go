// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
)

var _ forge.Wiki = (*Adapter)(nil)

type wikiWriteOptions struct {
	Title         string `json:"title"`
	ContentBase64 string `json:"content_base64"`
	Message       string `json:"message"`
}

func wikiRepoPath(owner, repo string) string {
	return forge.APIPath("repos", owner, repo, "wiki")
}

// wikiPageSegment makes a page name one escaped path segment, whether the
// caller passed it plain or already percent-encoded.
func wikiPageSegment(pageName string) string {
	decoded, err := url.PathUnescape(pageName)
	if err != nil {
		decoded = pageName
	}
	return url.PathEscape(decoded)
}

func (a *Adapter) ListWikiPages(ctx context.Context, owner, repo string, page, limit int) (forge.WikiPageList, error) {
	path := fmt.Sprintf("%s/pages?page=%d&limit=%d", wikiRepoPath(owner, repo), page, limit)
	var pages []forge.WikiPageMeta
	header, err := a.rawJSON(ctx, http.MethodGet, path, nil, &pages)
	if forge.IsNotFound(err) {
		return forge.WikiPageList{Pages: []forge.WikiPageMeta{}, TotalCount: totalCount(header)}, nil
	}
	if err != nil {
		return forge.WikiPageList{}, err
	}
	if pages == nil {
		pages = []forge.WikiPageMeta{}
	}
	return forge.WikiPageList{Pages: pages, TotalCount: totalCount(header)}, nil
}

func (a *Adapter) GetWikiPage(ctx context.Context, owner, repo, pageName string) (*forge.WikiPage, error) {
	var page forge.WikiPage
	path := fmt.Sprintf("%s/page/%s", wikiRepoPath(owner, repo), wikiPageSegment(pageName))
	if _, err := a.rawJSON(ctx, http.MethodGet, path, nil, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

func (a *Adapter) GetWikiPageRevisions(ctx context.Context, owner, repo, pageName string, page, limit int) (*forge.WikiRevisions, error) {
	var revisions forge.WikiRevisions
	path := fmt.Sprintf("%s/revisions/%s?page=%d&limit=%d", wikiRepoPath(owner, repo), wikiPageSegment(pageName), page, limit)
	if _, err := a.rawJSON(ctx, http.MethodGet, path, nil, &revisions); err != nil {
		return nil, err
	}
	return &revisions, nil
}

func (a *Adapter) CreateWikiPage(ctx context.Context, owner, repo, title, contentBase64, message string) (*forge.WikiPage, error) {
	var page forge.WikiPage
	body := wikiWriteOptions{Title: title, ContentBase64: contentBase64, Message: message}
	if _, err := a.rawJSON(ctx, http.MethodPost, wikiRepoPath(owner, repo)+"/new", body, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

func (a *Adapter) EditWikiPage(ctx context.Context, owner, repo, pageName, title, contentBase64, message string) (*forge.WikiPage, error) {
	var page forge.WikiPage
	body := wikiWriteOptions{Title: title, ContentBase64: contentBase64, Message: message}
	path := fmt.Sprintf("%s/page/%s", wikiRepoPath(owner, repo), wikiPageSegment(pageName))
	if _, err := a.rawJSON(ctx, http.MethodPatch, path, body, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

func (a *Adapter) DeleteWikiPage(ctx context.Context, owner, repo, pageName string) error {
	path := fmt.Sprintf("%s/page/%s", wikiRepoPath(owner, repo), wikiPageSegment(pageName))
	_, err := a.rawJSON(ctx, http.MethodDelete, path, nil, nil)
	return err
}
