// SPDX-License-Identifier: GPL-3.0-or-later

package wiki_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	appwiki "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/wiki"
)

func runSearch(t *testing.T, s *appwiki.Service, query string) model.WikiSearchResult {
	t.Helper()
	res, err := s.Search(context.Background(), appwiki.SearchRequest{Owner: owner, Repo: repo, Query: query})
	if err != nil {
		t.Fatalf("search %q: %v", query, err)
	}
	return res
}

func TestSearchMatchesTitleAndBody(t *testing.T) {
	t.Parallel()
	s := newService(t)
	create(t, s, "Pagination", "Cursors are opaque.\nUse limit.")

	byTitle := runSearch(t, s, "pagination")
	if len(byTitle.Hits) != 1 || byTitle.Hits[0].PageName != "Pagination" || byTitle.Hits[0].Line != 0 {
		t.Fatalf("title match: got %+v", byTitle.Hits)
	}

	byBody := runSearch(t, s, "cursors")
	if len(byBody.Hits) != 1 || byBody.Hits[0].PageName != "Pagination" || byBody.Hits[0].Line != 1 {
		t.Fatalf("body match: got %+v", byBody.Hits)
	}
}

func TestSearchIsCaseInsensitive(t *testing.T) {
	t.Parallel()
	s := newService(t)
	create(t, s, "Notes", "Cursors are opaque.")
	if got := runSearch(t, s, "CURSORS"); len(got.Hits) != 1 {
		t.Fatalf("case-insensitive body match: got %+v", got.Hits)
	}
}

func TestSearchRefusesEmptyQuery(t *testing.T) {
	t.Parallel()
	s := newService(t)
	create(t, s, "Notes", "text")
	if _, err := s.Search(context.Background(), appwiki.SearchRequest{Owner: owner, Repo: repo, Query: "   "}); err == nil {
		t.Fatal("a blank query was accepted")
	}
}

func TestSearchOnRepoWithNoWikiIsEmpty(t *testing.T) {
	t.Parallel()
	s := newService(t)
	got := runSearch(t, s, "anything")
	if len(got.Hits) != 0 || got.PagesScanned != 0 || got.Truncated {
		t.Fatalf("no wiki: got %+v", got)
	}
}

func TestSearchPageCapBoundary(t *testing.T) {
	t.Parallel()

	exact := newService(t)
	for i := 1; i <= appwiki.SearchPageCap; i++ {
		create(t, exact, fmt.Sprintf("Page %03d", i), "plain")
	}
	got := runSearch(t, exact, "needle")
	if got.Truncated || got.PagesScanned != appwiki.SearchPageCap {
		t.Fatalf("a wiki of exactly the cap: truncated=%v scanned=%d", got.Truncated, got.PagesScanned)
	}

	over := newService(t)
	for i := 1; i <= appwiki.SearchPageCap+1; i++ {
		body := "plain"
		if i == appwiki.SearchPageCap+1 {
			body = "needle"
		}
		create(t, over, fmt.Sprintf("Page %03d", i), body)
	}
	got = runSearch(t, over, "needle")
	if !got.Truncated || got.PagesScanned != appwiki.SearchPageCap || len(got.Hits) != 0 {
		t.Fatalf("a wiki over the cap: truncated=%v scanned=%d hits=%d", got.Truncated, got.PagesScanned, len(got.Hits))
	}
}

func TestSearchHitCapBoundary(t *testing.T) {
	t.Parallel()

	exact := newService(t)
	create(t, exact, "Log", strings.Repeat("entry\n", appwiki.SearchHitCap))
	got := runSearch(t, exact, "entry")
	if got.Truncated || len(got.Hits) != appwiki.SearchHitCap {
		t.Fatalf("exactly the hit cap: truncated=%v hits=%d", got.Truncated, len(got.Hits))
	}

	over := newService(t)
	create(t, over, "Log", strings.Repeat("entry\n", appwiki.SearchHitCap+1))
	got = runSearch(t, over, "entry")
	if !got.Truncated || len(got.Hits) != appwiki.SearchHitCap {
		t.Fatalf("over the hit cap: truncated=%v hits=%d", got.Truncated, len(got.Hits))
	}
}

func TestSearchSkipsOversizedPageAndCountsIt(t *testing.T) {
	t.Parallel()
	s := newService(t)
	create(t, s, "Big", strings.Repeat("needle ", appwiki.SearchMaxPageBytes/7+1))
	create(t, s, "Small", "needle here")

	got := runSearch(t, s, "needle")
	if got.PagesSkipped != 1 || got.PagesScanned != 2 || len(got.Hits) != 1 || got.Hits[0].PageName != "Small" {
		t.Fatalf("oversized page: skipped=%d scanned=%d hits=%+v", got.PagesSkipped, got.PagesScanned, got.Hits)
	}
}

func TestSearchSnippetIsBounded(t *testing.T) {
	t.Parallel()
	s := newService(t)
	long := strings.Repeat("a", 2500) + "needle" + strings.Repeat("b", 2500)
	create(t, s, "Long", long)

	got := runSearch(t, s, "needle")
	if len(got.Hits) != 1 {
		t.Fatalf("hits: got %+v", got.Hits)
	}
	if n := len(got.Hits[0].Snippet); n > appwiki.SearchSnippetBytes {
		t.Fatalf("snippet is %d bytes, over the %d-byte bound", n, appwiki.SearchSnippetBytes)
	}
	if !strings.Contains(got.Hits[0].Snippet, "needle") {
		t.Fatalf("snippet lost the match: %q", got.Hits[0].Snippet)
	}
}

// Some runes change byte length when lowered: U+0130 grows from 2 bytes to 3
// and U+1E9E shrinks from 3 to 2, and an invalid byte becomes U+FFFD. The
// snippet must centre on the match in the original line, not on its offset in
// the lowered copy, so the matched text stays in the snippet.
func TestSearchSnippetFindsMatchAfterLengthChangingRunes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, prefix string }{
		{"lengthening", strings.Repeat("İ", 100)},
		{"shortening", strings.Repeat("ẞ", 100)},
		{"invalid bytes", strings.Repeat("\xff", 100)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := newService(t)
			create(t, s, "Long", tc.prefix+"needle"+strings.Repeat("b", 2500))

			got := runSearch(t, s, "needle")
			if len(got.Hits) != 1 {
				t.Fatalf("hits: got %+v", got.Hits)
			}
			if !strings.Contains(got.Hits[0].Snippet, "needle") {
				t.Fatalf("snippet lost the match: %q", got.Hits[0].Snippet)
			}
		})
	}
}
