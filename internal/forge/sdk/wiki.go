// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge"
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

// GetWikiPageRevisions returns one page of a page's history. Forgejo 16.0.4
// ignores the limit query and pages the history by a size of its own, so the
// adapter reads whole Forgejo pages until the requested window is covered and
// cuts the window itself. Commit messages arrive with a trailing newline, which
// is trimmed.
func (a *Adapter) GetWikiPageRevisions(ctx context.Context, owner, repo, pageName string, page, limit int) (*forge.WikiRevisions, error) {
	base, err := forge.APIPath("repos", owner, repo, "wiki", "revisions", wikiPageName(pageName))
	if err != nil {
		return nil, err
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 30
	}
	lo, hi := (page-1)*limit, page*limit
	var all []forge.WikiCommit
	total := 0
	for upstream := 1; ; upstream++ {
		var got forge.WikiRevisions
		if _, err := a.rawJSON(ctx, http.MethodGet, fmt.Sprintf("%s?page=%d", base, upstream), nil, &got); err != nil {
			return nil, err
		}
		total = got.Count
		all = append(all, got.Commits...)
		if len(got.Commits) == 0 || len(all) >= total || len(all) >= hi {
			break
		}
	}
	if lo > len(all) {
		lo = len(all)
	}
	if hi > len(all) {
		hi = len(all)
	}
	commits := append([]forge.WikiCommit{}, all[lo:hi]...)
	for i := range commits {
		commits[i].Message = strings.TrimRight(commits[i].Message, "\r\n")
	}
	return &forge.WikiRevisions{Commits: commits, Count: total}, nil
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
