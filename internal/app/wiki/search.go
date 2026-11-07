// SPDX-License-Identifier: GPL-3.0-or-later

package wiki

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode"
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
	if start, end, ok := matchSpan(meta.Title, needle); ok {
		if addHit(res, model.WikiSearchHit{PageName: meta.SubURL, Snippet: snippetOf(meta.Title, start, end)}) {
			return true
		}
	}
	for i, line := range strings.Split(content, "\n") {
		start, end, ok := matchSpan(line, needle)
		if !ok {
			continue
		}
		if addHit(res, model.WikiSearchHit{PageName: meta.SubURL, Line: i + 1, Snippet: snippetOf(line, start, end)}) {
			return true
		}
	}
	return false
}

// matchSpan finds the lowered needle in line and returns the match as byte
// offsets into line itself. The search runs on strings.ToLower(line), whose
// offsets differ from line's where a rune changes length, so the offsets are
// mapped back rune by rune.
func matchSpan(line, needle string) (start, end int, ok bool) {
	idx := strings.Index(strings.ToLower(line), needle)
	if idx < 0 {
		return 0, 0, false
	}
	return loweredToOriginal(line, idx), loweredToOriginal(line, idx+len(needle)), true
}

// loweredToOriginal maps a byte offset in strings.ToLower(s) to the byte offset
// in s of the rune that produced it. strings.ToLower maps rune by rune, so the
// lowered offset of each rune boundary is the running sum of the lowered rune
// lengths; the result is clamped to len(s).
func loweredToOriginal(s string, lowered int) int {
	acc := 0
	for i, r := range s {
		if acc >= lowered {
			return i
		}
		acc += utf8.RuneLen(unicode.ToLower(r))
	}
	return len(s)
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

// snippetOf returns at most SearchSnippetBytes of line, centred on the match
// line[matchStart:matchEnd] where the line is longer than the bound. Its ends
// fall on rune boundaries.
func snippetOf(line string, matchStart, matchEnd int) string {
	if len(line) <= SearchSnippetBytes {
		return strings.TrimSpace(line)
	}
	start := matchStart - (SearchSnippetBytes-(matchEnd-matchStart))/2
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
