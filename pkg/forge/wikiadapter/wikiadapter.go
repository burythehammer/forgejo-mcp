// SPDX-License-Identifier: GPL-3.0-or-later

// Package wikiadapter implements the wiki use cases' port (app/wiki.Port) over
// the Forgejo port (forge.Wiki). The forge and repo-owned wiki types are plain
// data of the same shape, so each is mapped field by field here: the one place
// the two are joined.
package wikiadapter

import (
	"context"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
	appwiki "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/wiki"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
)

// New returns the use cases' port over p.
func New(p forge.Wiki) appwiki.Port { return adapter{p: p} }

type adapter struct{ p forge.Wiki }

var _ appwiki.Port = adapter{}

func person(p forge.WikiPerson) model.WikiPerson {
	return model.WikiPerson{Name: p.Name, Email: p.Email, Date: p.Date}
}

func commit(c forge.WikiCommit) model.WikiCommit {
	return model.WikiCommit{SHA: c.SHA, Author: person(c.Author), Message: c.Message}
}

func meta(m forge.WikiPageMeta) model.WikiPageMeta {
	return model.WikiPageMeta{Title: m.Title, HTMLURL: m.HTMLURL, SubURL: m.SubURL, LastCommit: commit(m.LastCommit)}
}

func page(p *forge.WikiPage) *model.WikiPage {
	if p == nil {
		return nil
	}
	return &model.WikiPage{
		WikiPageMeta:  meta(p.WikiPageMeta),
		ContentBase64: p.ContentBase64,
		CommitCount:   p.CommitCount,
		Sidebar:       p.Sidebar,
		Footer:        p.Footer,
	}
}

func (a adapter) ListWikiPages(ctx context.Context, owner, repo string, page, limit int) (model.WikiPageList, error) {
	list, err := a.p.ListWikiPages(ctx, owner, repo, page, limit)
	if err != nil {
		return model.WikiPageList{}, err
	}
	pages := make([]model.WikiPageMeta, len(list.Pages))
	for i, p := range list.Pages {
		pages[i] = meta(p)
	}
	return model.WikiPageList{Pages: pages, TotalCount: list.TotalCount}, nil
}

func (a adapter) GetWikiPage(ctx context.Context, owner, repo, pageName string) (*model.WikiPage, error) {
	p, err := a.p.GetWikiPage(ctx, owner, repo, pageName)
	if err != nil {
		return nil, err
	}
	return page(p), nil
}

func (a adapter) GetWikiPageRevisions(ctx context.Context, owner, repo, pageName string, page, limit int) (*model.WikiRevisions, error) {
	r, err := a.p.GetWikiPageRevisions(ctx, owner, repo, pageName, page, limit)
	if err != nil {
		return nil, err
	}
	commits := make([]model.WikiCommit, len(r.Commits))
	for i, c := range r.Commits {
		commits[i] = commit(c)
	}
	return &model.WikiRevisions{Commits: commits, Count: r.Count}, nil
}

func (a adapter) CreateWikiPage(ctx context.Context, owner, repo, title, contentBase64, message string) (*model.WikiPage, error) {
	p, err := a.p.CreateWikiPage(ctx, owner, repo, title, contentBase64, message)
	if err != nil {
		return nil, err
	}
	return page(p), nil
}

func (a adapter) EditWikiPage(ctx context.Context, owner, repo, pageName, title, contentBase64, message string) (*model.WikiPage, error) {
	p, err := a.p.EditWikiPage(ctx, owner, repo, pageName, title, contentBase64, message)
	if err != nil {
		return nil, err
	}
	return page(p), nil
}

func (a adapter) DeleteWikiPage(ctx context.Context, owner, repo, pageName string) error {
	return a.p.DeleteWikiPage(ctx, owner, repo, pageName)
}
