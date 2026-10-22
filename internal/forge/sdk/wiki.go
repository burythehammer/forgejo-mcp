// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
)

var _ forge.Wiki = (*Adapter)(nil)

type wikiWriteOptions struct {
	Title         string `json:"title"`
	ContentBase64 string `json:"content_base64"`
	Message       string `json:"message"`
}

// wikiPageName makes a page name the plain text of one path segment, whether
// the caller passed it plain or already percent-encoded. forge.APIPath escapes
// it, and refuses a name that is "." or "..".
func wikiPageName(pageName string) string {
	decoded, err := url.PathUnescape(pageName)
	if err != nil {
		return pageName
	}
	return decoded
}

func (a *Adapter) ListWikiPages(ctx context.Context, owner, repo string, page, limit int) (forge.WikiPageList, error) {
	base, err := forge.APIPath("repos", owner, repo, "wiki")
	if err != nil {
		return forge.WikiPageList{}, err
	}
	path := fmt.Sprintf("%s/pages?page=%d&limit=%d", base, page, limit)
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
	path, err := forge.APIPath("repos", owner, repo, "wiki", "page", wikiPageName(pageName))
	if err != nil {
		return nil, err
	}
	if _, err := a.rawJSON(ctx, http.MethodGet, path, nil, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

func (a *Adapter) GetWikiPageRevisions(ctx context.Context, owner, repo, pageName string, page, limit int) (*forge.WikiRevisions, error) {
	var revisions forge.WikiRevisions
	base, err := forge.APIPath("repos", owner, repo, "wiki", "revisions", wikiPageName(pageName))
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("%s?page=%d&limit=%d", base, page, limit)
	if _, err := a.rawJSON(ctx, http.MethodGet, path, nil, &revisions); err != nil {
		return nil, err
	}
	return &revisions, nil
}

func (a *Adapter) CreateWikiPage(ctx context.Context, owner, repo, title, contentBase64, message string) (*forge.WikiPage, error) {
	var page forge.WikiPage
	body := wikiWriteOptions{Title: title, ContentBase64: contentBase64, Message: message}
	path, err := forge.APIPath("repos", owner, repo, "wiki", "new")
	if err != nil {
		return nil, err
	}
	if _, err := a.rawJSON(ctx, http.MethodPost, path, body, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

func (a *Adapter) EditWikiPage(ctx context.Context, owner, repo, pageName, title, contentBase64, message string) (*forge.WikiPage, error) {
	var page forge.WikiPage
	body := wikiWriteOptions{Title: title, ContentBase64: contentBase64, Message: message}
	path, err := forge.APIPath("repos", owner, repo, "wiki", "page", wikiPageName(pageName))
	if err != nil {
		return nil, err
	}
	if _, err := a.rawJSON(ctx, http.MethodPatch, path, body, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

func (a *Adapter) DeleteWikiPage(ctx context.Context, owner, repo, pageName string) error {
	path, err := forge.APIPath("repos", owner, repo, "wiki", "page", wikiPageName(pageName))
	if err != nil {
		return err
	}
	_, err = a.rawJSON(ctx, http.MethodDelete, path, nil, nil)
	return err
}
