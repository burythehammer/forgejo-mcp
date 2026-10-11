// SPDX-License-Identifier: GPL-3.0-or-later

package wiki

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/lines"
)

// defaultLimit is the page size when the caller gives none.
const defaultLimit = 30

// Service runs the wiki use cases over a Port. Build one per request from the
// port the request's credential selects; it holds no other state.
type Service struct{ port Port }

// New returns the use cases bound to port.
func New(port Port) *Service { return &Service{port: port} }

// ListRequest lists a wiki's pages. Page and Limit are the numbers as the
// caller gave them: a value that is not positive is the default (page 1, 30
// pages), and a positive fraction truncates.
type ListRequest struct {
	Owner, Repo string
	Page, Limit float64
}

// GetRequest reads a page. StartLine and EndLine are optional line numbers; a
// nil one means it was not given.
type GetRequest struct {
	Owner, Repo, PageName string
	StartLine, EndLine    *float64
}

// RevisionsRequest lists a page's history, paged as ListRequest is.
type RevisionsRequest struct {
	Owner, Repo, PageName string
	Page, Limit           float64
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

// paging reads a page and a limit as the tools always have: a value that is
// not positive is the default, and a positive value is truncated to a whole
// number.
func paging(page, limit float64) (int, int) {
	p, l := 1, defaultLimit
	if page > 0 {
		p = int(page)
	}
	if limit > 0 {
		l = int(limit)
	}
	return p, l
}

// List returns one page of a wiki's pages, and whether another page follows.
func (s *Service) List(ctx context.Context, r ListRequest) (model.WikiPageListResult, error) {
	page, limit := paging(r.Page, r.Limit)
	listed, err := s.port.ListWikiPages(ctx, r.Owner, r.Repo, page, limit)
	if err != nil {
		return model.WikiPageListResult{}, fmt.Errorf("list wiki pages: %w", err)
	}
	pages := listed.Pages
	hasNext := false
	if len(pages) == limit {
		next, err := s.port.ListWikiPages(ctx, r.Owner, r.Repo, page+1, limit)
		if err != nil {
			return model.WikiPageListResult{}, fmt.Errorf("probe next wiki page: %w", err)
		}
		hasNext = len(next.Pages) > 0
	}
	summaries := make([]model.WikiPageSummary, len(pages))
	for i, p := range pages {
		summaries[i] = model.WikiPageSummary{Title: p.Title, PageName: p.SubURL, SubURL: p.SubURL}
	}
	return model.WikiPageListResult{Pages: summaries, Page: page, HasNext: hasNext, TotalCount: listed.TotalCount}, nil
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
		var from, to float64
		if r.StartLine != nil {
			from = *r.StartLine
		}
		if r.EndLine != nil {
			to = *r.EndLine
		}
		start, end = int(from), int(to)
		content, err = lines.Slice(content, start, end)
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
func (s *Service) Revisions(ctx context.Context, r RevisionsRequest) (model.WikiRevisionsResult, error) {
	page, limit := paging(r.Page, r.Limit)
	revisions, err := s.port.GetWikiPageRevisions(ctx, r.Owner, r.Repo, r.PageName, page, limit)
	if err != nil {
		return model.WikiRevisionsResult{}, fmt.Errorf("get wiki revisions: %w", err)
	}
	hasNext := false
	if len(revisions.Commits) == limit {
		next, err := s.port.GetWikiPageRevisions(ctx, r.Owner, r.Repo, r.PageName, page+1, limit)
		if err != nil {
			return model.WikiRevisionsResult{}, fmt.Errorf("probe next wiki revision page: %w", err)
		}
		hasNext = len(next.Commits) > 0
	}
	summaries := make([]model.WikiRevisionSummary, len(revisions.Commits))
	for i, c := range revisions.Commits {
		summaries[i] = model.WikiRevisionSummary{SHA: c.SHA, Author: c.Author.Name, Message: c.Message}
	}
	// The total is the page's revision count from the body, which the port
	// reports as Count.
	return model.WikiRevisionsResult{Revisions: summaries, Page: page, HasNext: hasNext, TotalCount: revisions.Count}, nil
}

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
