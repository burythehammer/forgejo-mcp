// SPDX-License-Identifier: GPL-3.0-or-later

package fake

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var _ forge.Wiki = (*Forgejo)(nil)

type wikiState struct {
	mu    sync.Mutex
	wikis map[string]map[string]*wikiRecord // repo key -> normalized name -> page
	seq   int
}

type wikiRecord struct {
	title   string
	content string             // base64, as the API carries it
	history []forge.WikiCommit // newest first
}

// wikiKey is the page's server-normalized name: spaces become dashes. Callers
// may pass it percent-encoded.
func wikiKey(name string) string {
	if decoded, err := url.PathUnescape(name); err == nil {
		name = decoded
	}
	return strings.ReplaceAll(name, " ", "-")
}

// wikiTitleKey is the page name a title is stored under. Forgejo 16.0.4 stores
// a title that contains a hyphen as "<title>.-", so a read by the plain title
// is not found and only the name the create answered works (#388).
func wikiTitleKey(title string) string {
	key := wikiKey(title)
	if strings.Contains(title, "-") {
		key += ".-"
	}
	return key
}

func (f *Forgejo) wikiCommit(message string) forge.WikiCommit {
	f.wiki.seq++
	sum := sha1.Sum([]byte(fmt.Sprint("wiki-commit-", f.wiki.seq)))
	return forge.WikiCommit{
		SHA:     hex.EncodeToString(sum[:]),
		Author:  forge.WikiPerson{Name: Poster},
		Message: message,
	}
}

func (r *wikiRecord) meta(key string) forge.WikiPageMeta {
	m := forge.WikiPageMeta{Title: r.title, SubURL: url.PathEscape(key)}
	if len(r.history) > 0 {
		m.LastCommit = r.history[0]
	}
	return m
}

func (r *wikiRecord) page(key string) *forge.WikiPage {
	return &forge.WikiPage{WikiPageMeta: r.meta(key), ContentBase64: r.content, CommitCount: len(r.history)}
}

// wikiPages returns the repository's pages, or a 404 when the repository does
// not exist. It must be called with the wiki lock held; the repository registry
// has its own lock, taken inside.
func (f *Forgejo) wikiPages(owner, repo string) (map[string]*wikiRecord, error) {
	if !f.repoExists(owner, repo) {
		return nil, notFound("repository")
	}
	if f.wiki.wikis == nil {
		f.wiki.wikis = map[string]map[string]*wikiRecord{}
	}
	k := owner + "/" + repo
	if f.wiki.wikis[k] == nil {
		f.wiki.wikis[k] = map[string]*wikiRecord{}
	}
	return f.wiki.wikis[k], nil
}

func (f *Forgejo) ListWikiPages(_ context.Context, owner, repo string, page, limit int) (forge.WikiPageList, error) {
	f.wiki.mu.Lock()
	defer f.wiki.mu.Unlock()
	pages, err := f.wikiPages(owner, repo)
	if err != nil {
		// Forgejo answers 404 for a wiki that does not exist; the port reports
		// that as an empty list.
		return forge.WikiPageList{Pages: []forge.WikiPageMeta{}}, nil
	}
	keys := make([]string, 0, len(pages))
	for k := range pages {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	lo, hi := listPage(page, limit, len(keys))
	out := make([]forge.WikiPageMeta, 0, hi-lo)
	for _, k := range keys[lo:hi] {
		out = append(out, pages[k].meta(k))
	}
	total := len(keys)
	return forge.WikiPageList{Pages: out, TotalCount: &total}, nil
}

// listPage is the [lo, hi) window of page/limit over n items.
func listPage(pageNo, limit, n int) (lo, hi int) {
	return page(forgejo_sdk.ListOptions{Page: pageNo, PageSize: limit}, n)
}

func (f *Forgejo) GetWikiPage(_ context.Context, owner, repo, pageName string) (*forge.WikiPage, error) {
	f.wiki.mu.Lock()
	defer f.wiki.mu.Unlock()
	pages, err := f.wikiPages(owner, repo)
	if err != nil {
		return nil, err
	}
	key := wikiKey(pageName)
	rec, ok := pages[key]
	if !ok {
		return nil, notFound("wiki page")
	}
	return rec.page(key), nil
}

func (f *Forgejo) GetWikiPageRevisions(_ context.Context, owner, repo, pageName string, page, limit int) (*forge.WikiRevisions, error) {
	f.wiki.mu.Lock()
	defer f.wiki.mu.Unlock()
	pages, err := f.wikiPages(owner, repo)
	if err != nil {
		return nil, err
	}
	rec, ok := pages[wikiKey(pageName)]
	if !ok {
		return nil, notFound("wiki page")
	}
	lo, hi := listPage(page, limit, len(rec.history))
	return &forge.WikiRevisions{Commits: append([]forge.WikiCommit{}, rec.history[lo:hi]...), Count: len(rec.history)}, nil
}

func (f *Forgejo) CreateWikiPage(_ context.Context, owner, repo, title, contentBase64, message string) (*forge.WikiPage, error) {
	f.wiki.mu.Lock()
	defer f.wiki.mu.Unlock()
	pages, err := f.wikiPages(owner, repo)
	if err != nil {
		return nil, err
	}
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}
	key := wikiTitleKey(title)
	if _, ok := pages[key]; ok {
		// Forgejo refuses a create of an existing page with a 400, as the live
		// contract run against 16.0.4 showed.
		return nil, statusErr(http.StatusBadRequest, "wiki page already exists [title: "+key+".md]")
	}
	rec := &wikiRecord{title: title, content: contentBase64}
	pages[key] = rec
	rec.history = append([]forge.WikiCommit{f.wikiCommit(message)}, rec.history...)
	return rec.page(key), nil
}

func (f *Forgejo) EditWikiPage(_ context.Context, owner, repo, pageName, title, contentBase64, message string) (*forge.WikiPage, error) {
	f.wiki.mu.Lock()
	defer f.wiki.mu.Unlock()
	pages, err := f.wikiPages(owner, repo)
	if err != nil {
		return nil, err
	}
	oldKey := wikiKey(pageName)
	rec, ok := pages[oldKey]
	if !ok {
		// Forgejo's edit of a page that does not exist creates it (live run
		// against 16.0.4), so the fake does the same.
		rec = &wikiRecord{}
	}
	if title == "" {
		title = rec.title
	}
	if title == "" {
		title = pageName
	}
	key := wikiTitleKey(title)
	if key != oldKey {
		delete(pages, oldKey)
	}
	pages[key] = rec
	rec.title, rec.content = title, contentBase64
	rec.history = append([]forge.WikiCommit{f.wikiCommit(message)}, rec.history...)
	return rec.page(key), nil
}

func (f *Forgejo) DeleteWikiPage(_ context.Context, owner, repo, pageName string) error {
	f.wiki.mu.Lock()
	defer f.wiki.mu.Unlock()
	pages, err := f.wikiPages(owner, repo)
	if err != nil {
		return err
	}
	key := wikiKey(pageName)
	if _, ok := pages[key]; !ok {
		return notFound("wiki page")
	}
	delete(pages, key)
	return nil
}
