// SPDX-License-Identifier: GPL-3.0-or-later

package model

// WikiSearchHit is one place a search_wiki query matched: a page title, which
// has no line, or one line of a page's body.
type WikiSearchHit struct {
	PageName string `json:"page_name"`
	Line     int    `json:"line,omitempty"`
	Snippet  string `json:"snippet"`
}

// WikiSearchResult is the search_wiki output. PagesSkipped counts the pages
// that were read but not searched (too large, or unreadable); they are also
// counted in PagesScanned.
type WikiSearchResult struct {
	Hits         []WikiSearchHit `json:"hits"`
	PagesScanned int             `json:"pages_scanned"`
	PagesSkipped int             `json:"pages_skipped"`
	Truncated    bool            `json:"truncated"`
}
