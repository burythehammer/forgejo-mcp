// SPDX-License-Identifier: GPL-3.0-or-later

package wiki

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/list"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	apprepo "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/repo"
)

// defaultLimit is the page size when the caller gives none.
const defaultLimit = 30

// Service runs the wiki use cases over a Port. Build one per request from the
// port the request's credential selects; it holds no other state.
type Service struct{ port Port }

// New returns the use cases bound to port.
func New(port Port) *Service { return &Service{port: port} }

// ListRequest lists a wiki's pages. Page and Limit are whole numbers the
// adapter has already validated; a value below 1 is the default (page 1, 30
// pages).
type ListRequest struct {
	Owner, Repo string
	Page, Limit int
}

// GetRequest reads a page. StartLine and EndLine are optional line numbers; a
// nil one means it was not given.
type GetRequest struct {
	Owner, Repo, PageName string
	StartLine, EndLine    *int
}

// RevisionsRequest lists a page's history, paged as ListRequest is.
type RevisionsRequest struct {
	Owner, Repo, PageName string
	Page, Limit           int
}

// CreateRequest creates a page. An empty Message gets a default.
type CreateRequest struct {
	Owner, Repo, Title, Content, Message string
}

// UpdateRequest edits a page. An empty Title keeps the page's title; an empty
// Message gets a default.
type UpdateRequest struct {
	Owner, Repo, PageName, Title, Content, Message string
}

// PageRef names one page of a wiki.
type PageRef struct {
	Owner, Repo, PageName string
}

// paging applies the defaults to a validated page and limit: a value below 1 is
// the default.
func paging(page, limit int) (int, int) {
	p, l := 1, defaultLimit
	if page > 0 {
		p = page
	}
	if limit > 0 {
		l = limit
	}
	return p, l
}

// List returns one page of a wiki's pages with its next marker. The ceiling
// check and the default limit come from the list module, and so does the
// continuation probe. The X-Total-Count the wiki reports counts raw tree
// entries, not pages, so it is not a grand total: it is neither reported as
// total_count nor passed to the module.
func (s *Service) List(ctx context.Context, r ListRequest) (list.Bounded[model.WikiPageSummary], error) {
	ceiling, known := s.port.MaxResponseItems(ctx)
	c := list.Ceiling{Max: ceiling, Known: known}
	return list.Run(ctx, list.Request{Page: r.Page, Limit: r.Limit, Def: defaultLimit}, c,
		func(ctx context.Context, p list.Paging) (list.Batch[model.WikiPageSummary], error) {
			listed, err := s.port.ListWikiPages(ctx, r.Owner, r.Repo, p.Page, p.Limit)
			if err != nil {
				return list.Batch[model.WikiPageSummary]{}, fmt.Errorf("list wiki pages: %w", err)
			}
			return list.Batch[model.WikiPageSummary]{Items: wikiSummaries(listed.Pages)}, nil
		})
}

// wikiSummaries is each listed wiki page in its summary form.
func wikiSummaries(pages []model.WikiPageMeta) []model.WikiPageSummary {
	out := make([]model.WikiPageSummary, len(pages))
	for i, p := range pages {
		out[i] = model.WikiPageSummary{Title: p.Title, PageName: p.SubURL, SubURL: p.SubURL}
	}
	return out
}

// Get returns a page's content, cut to the requested line range when one is
// given. Line numbers are clamped; an inverted range is an error.
func (s *Service) Get(ctx context.Context, r GetRequest) (model.WikiPageView, error) {
	page, err := s.port.GetWikiPage(ctx, r.Owner, r.Repo, r.PageName)
	if err != nil {
		return model.WikiPageView{}, fmt.Errorf("get wiki page: %w", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(page.ContentBase64)
	if err != nil {
		return model.WikiPageView{}, fmt.Errorf("decode wiki page content: %w", err)
	}
	content := string(decoded)
	total := len(strings.Split(content, "\n"))
	start, end := 0, 0
	if r.StartLine != nil || r.EndLine != nil {
		if r.StartLine != nil {
			start = *r.StartLine
		}
		if r.EndLine != nil {
			end = *r.EndLine
		}
		content, err = apprepo.SliceLines(content, start, end)
		if err != nil {
			return model.WikiPageView{}, err
		}
		if start <= 0 {
			start = 1
		}
		if end <= 0 || end > total {
			end = total
		}
	}
	return model.WikiPageView{
		Title: page.Title, PageName: page.SubURL, Content: content, CommitSHA: page.LastCommit.SHA,
		TotalLines: total, StartLine: start, EndLine: end,
	}, nil
}

// Revisions returns one page of a page's history, and whether another follows.
func (s *Service) Revisions(ctx context.Context, r RevisionsRequest) (list.Bounded[model.WikiRevisionSummary], error) {
	page, limit := paging(r.Page, r.Limit)
	// The history has never been held to the instance ceiling, so the window
	// reads through a source that reports none.
	return list.Window(ctx, noCeiling{}, list.NewRequest(list.Paging{Page: page, Limit: limit}, limit),
		func(ctx context.Context, q list.Paging) (list.Batch[model.WikiRevisionSummary], error) {
			revisions, err := s.port.GetWikiPageRevisions(ctx, r.Owner, r.Repo, r.PageName, q.Page, q.Limit)
			if err != nil {
				return list.Batch[model.WikiRevisionSummary]{}, fmt.Errorf("get wiki revisions: %w", err)
			}
			// A full page may be the last, so the next page is probed once. The
			// body's count is the grand total, reported as Total.
			more := false
			if len(revisions.Commits) == q.Limit {
				next, err := s.port.GetWikiPageRevisions(ctx, r.Owner, r.Repo, r.PageName, q.Page+1, q.Limit)
				if err != nil {
					return list.Batch[model.WikiRevisionSummary]{}, fmt.Errorf("probe next wiki revision page: %w", err)
				}
				more = len(next.Commits) > 0
			}
			summaries := make([]model.WikiRevisionSummary, len(revisions.Commits))
			for i, c := range revisions.Commits {
				summaries[i] = model.WikiRevisionSummary{SHA: c.SHA, Author: c.Author.Name, Message: c.Message}
			}
			return list.Batch[model.WikiRevisionSummary]{Items: summaries, Total: list.Reported(revisions.Count, true), More: more}, nil
		})
}

// noCeiling is the ceiling source of a listing the instance ceiling does not apply to.
type noCeiling struct{}

// MaxResponseItems reports no ceiling.
func (noCeiling) MaxResponseItems(context.Context) (int, bool) { return 0, false }

// Create adds a page and returns its title, name and commit.
func (s *Service) Create(ctx context.Context, r CreateRequest) (model.WikiWriteResult, error) {
	message := r.Message
	if message == "" {
		message = fmt.Sprintf("Create wiki page '%s'", r.Title)
	}
	page, err := s.port.CreateWikiPage(ctx, r.Owner, r.Repo, r.Title, encode(r.Content), message)
	if err != nil {
		return model.WikiWriteResult{}, fmt.Errorf("create wiki page: %w", err)
	}
	return writeResult(page), nil
}

// Update edits a page and returns its title, name and commit. An empty title
// is read from the page first, so the title is preserved.
func (s *Service) Update(ctx context.Context, r UpdateRequest) (model.WikiWriteResult, error) {
	title := r.Title
	if title == "" {
		current, err := s.port.GetWikiPage(ctx, r.Owner, r.Repo, r.PageName)
		if err != nil {
			return model.WikiWriteResult{}, fmt.Errorf("read wiki page before update: %w", err)
		}
		title = current.Title
	}
	message := r.Message
	if message == "" {
		message = fmt.Sprintf("Update wiki page '%s'", r.PageName)
	}
	page, err := s.port.EditWikiPage(ctx, r.Owner, r.Repo, r.PageName, title, encode(r.Content), message)
	if err != nil {
		return model.WikiWriteResult{}, fmt.Errorf("update wiki page: %w", err)
	}
	return writeResult(page), nil
}

// Delete removes a page.
func (s *Service) Delete(ctx context.Context, r PageRef) (model.WikiDeleteResult, error) {
	if err := s.port.DeleteWikiPage(ctx, r.Owner, r.Repo, r.PageName); err != nil {
		return model.WikiDeleteResult{}, fmt.Errorf("delete wiki page: %w", err)
	}
	return model.WikiDeleteResult{Deleted: true, PageName: r.PageName}, nil
}

func encode(content string) string { return base64.StdEncoding.EncodeToString([]byte(content)) }

func writeResult(page *model.WikiPage) model.WikiWriteResult {
	return model.WikiWriteResult{Title: page.Title, PageName: page.SubURL, CommitSHA: page.LastCommit.SHA}
}
