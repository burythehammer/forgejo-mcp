// SPDX-License-Identifier: GPL-3.0-or-later

package wiki

import (
	"context"
	"encoding/base64"
	"fmt"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
)

// PageRead is one wiki page with the head of its history.
type PageRead struct {
	Title     string
	PageName  string
	CommitSHA string
	// Content is the whole page, decoded.
	Content string
	// Revisions holds at most the revision limit, newest first as Forgejo lists them.
	Revisions []model.WikiRevisionSummary
	// RevisionCount is the page's revision count as Forgejo reports it, and never
	// less than the revisions read.
	RevisionCount int
	// RevisionsTruncated is true when the page has more revisions than the limit.
	RevisionsTruncated bool
}

// ReadPage returns a page with its content and the first revisionLimit of its
// history. It is the read behind the wiki page resource; get_wiki_page and
// get_wiki_revisions read the same page and history through their own paging.
// A failed revision listing degrades the read to no history rather than failing
// it, as the resource always has: the page is still returned, untruncated.
func (s *Service) ReadPage(ctx context.Context, owner, repo, pageName string, revisionLimit int) (PageRead, error) {
	page, err := s.port.GetWikiPage(ctx, owner, repo, pageName)
	if err != nil {
		return PageRead{}, fmt.Errorf("get wiki page: %w", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(page.ContentBase64)
	if err != nil {
		return PageRead{}, fmt.Errorf("decode wiki page content: %w", err)
	}

	// One revision more than the limit, so that "more than the limit" is visible
	// without a second call.
	revs, err := s.port.GetWikiPageRevisions(ctx, owner, repo, pageName, 1, revisionLimit+1)
	if err != nil {
		return PageRead{Title: page.Title, PageName: page.SubURL, CommitSHA: page.LastCommit.SHA, Content: string(decoded), Revisions: []model.WikiRevisionSummary{}}, nil
	}
	shown := revs.Commits
	if len(shown) > revisionLimit {
		shown = shown[:revisionLimit]
	}
	summaries := make([]model.WikiRevisionSummary, len(shown))
	for i, c := range shown {
		summaries[i] = model.WikiRevisionSummary{SHA: c.SHA, Author: c.Author.Name, Message: c.Message}
	}
	count := revs.Count
	if n := len(revs.Commits); n > count {
		count = n
	}
	return PageRead{
		Title:              page.Title,
		PageName:           page.SubURL,
		CommitSHA:          page.LastCommit.SHA,
		Content:            string(decoded),
		Revisions:          summaries,
		RevisionCount:      count,
		RevisionsTruncated: len(revs.Commits) > revisionLimit || revs.Count > revisionLimit,
	}, nil
}
