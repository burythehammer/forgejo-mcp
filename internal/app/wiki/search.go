// SPDX-License-Identifier: GPL-3.0-or-later

package wiki

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
)

// The search bounds. They are constants in one place so they can change without
// a contract change; the tool description and the README row state them.
const (
	// SearchPageCap is the most pages a search reads.
	SearchPageCap = 100
	// SearchHitCap is the most hits a search returns.
	SearchHitCap = 20
	// SearchListLimit is the page size used to list the wiki's page titles.
	SearchListLimit = 30
	// SearchMaxPageBytes is the largest page a search reads; a larger one is
	// skipped. It matches the inline download cap in internal/forge.
	SearchMaxPageBytes = 1 << 20
	// SearchSnippetBytes is the most bytes of a matching line a hit carries.
	SearchSnippetBytes = 120
)

// SearchRequest is a plain-text search of one repository's wiki.
type SearchRequest struct {
	Owner, Repo, Query string
}

// Search scans the wiki's pages for the query, matching case-insensitively on
// each page's title and on each line of its body. It lists titles a batch at a
// time and reads a page only when it is about to scan it, so the cap stops the
// scan early on a large wiki. Truncated is set only when a page or a hit was
// left unscanned or uncollected, so a false value means every page was read.
// A page that cannot be read or is too large is skipped and counted; a failed
// title listing fails the call.
func (s *Service) Search(ctx context.Context, r SearchRequest) (model.WikiSearchResult, error) {
	if strings.TrimSpace(r.Query) == "" {
		return model.WikiSearchResult{}, errors.New("query is required")
	}
	needle := strings.ToLower(r.Query)
	res := model.WikiSearchResult{Hits: []model.WikiSearchHit{}}

	var queue []model.WikiPageMeta
	listPage := 1
	for {
		if len(queue) == 0 {
			batch, err := s.port.ListWikiPages(ctx, r.Owner, r.Repo, listPage, SearchListLimit)
			if err != nil {
				return model.WikiSearchResult{}, fmt.Errorf("list wiki pages: %w", err)
			}
			if len(batch.Pages) == 0 {
				return res, nil
			}
			queue = batch.Pages
			listPage++
		}
		// A queued page the cap leaves unscanned means the wiki is not fully read.
		if res.PagesScanned >= SearchPageCap {
			res.Truncated = true
			return res, nil
		}
		meta := queue[0]
		queue = queue[1:]
		res.PagesScanned++
		content, ok := s.readForSearch(ctx, r.Owner, r.Repo, meta.SubURL)
		if !ok {
			res.PagesSkipped++
			continue
		}
		if addPageHits(&res, meta, content, needle) {
			return res, nil
		}
	}
}

// readForSearch reads one page's decoded content. It reports false for a page
// that cannot be read or decoded, or that is too large to scan.
func (s *Service) readForSearch(ctx context.Context, owner, repo, pageName string) (string, bool) {
	page, err := s.port.GetWikiPage(ctx, owner, repo, pageName)
	if err != nil {
		return "", false
	}
	decoded, err := base64.StdEncoding.DecodeString(page.ContentBase64)
	if err != nil || len(decoded) > SearchMaxPageBytes {
		return "", false
	}
	return string(decoded), true
}

// addPageHits records the page's title match, then one hit per matching body
// line. It reports true when the hit cap is full and another hit exists, which
// ends the scan.
func addPageHits(res *model.WikiSearchResult, meta model.WikiPageMeta, content, needle string) bool {
	if idx := strings.Index(strings.ToLower(meta.Title), needle); idx >= 0 {
		if addHit(res, model.WikiSearchHit{PageName: meta.SubURL, Snippet: snippetOf(meta.Title, idx, len(needle))}) {
			return true
		}
	}
	for i, line := range strings.Split(content, "\n") {
		idx := strings.Index(strings.ToLower(line), needle)
		if idx < 0 {
			continue
		}
		if addHit(res, model.WikiSearchHit{PageName: meta.SubURL, Line: i + 1, Snippet: snippetOf(line, idx, len(needle))}) {
			return true
		}
	}
	return false
}

// addHit appends a hit. When the cap is already full it marks the result
// truncated and reports true, so the caller stops; a full cap with no further
// hit is not truncated.
func addHit(res *model.WikiSearchResult, hit model.WikiSearchHit) bool {
	if len(res.Hits) == SearchHitCap {
		res.Truncated = true
		return true
	}
	res.Hits = append(res.Hits, hit)
	return false
}

// snippetOf returns at most SearchSnippetBytes of line, centred on the match at
// idx where the line is longer than the bound. Its ends fall on rune boundaries.
func snippetOf(line string, idx, matchLen int) string {
	if len(line) <= SearchSnippetBytes {
		return strings.TrimSpace(line)
	}
	if idx > len(line) {
		idx = len(line)
	}
	start := idx - (SearchSnippetBytes-matchLen)/2
	if start < 0 {
		start = 0
	}
	end := start + SearchSnippetBytes
	if end > len(line) {
		end = len(line)
		start = end - SearchSnippetBytes
	}
	for start > 0 && !utf8.RuneStart(line[start]) {
		start--
	}
	for end < len(line) && end > start && !utf8.RuneStart(line[end]) {
		end--
	}
	return strings.TrimSpace(line[start:end])
}
