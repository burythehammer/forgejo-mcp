// SPDX-License-Identifier: GPL-3.0-or-later

package wiki

import (
	"context"
	"errors"

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

// Search is not implemented yet (red step of the test-first build).
func (s *Service) Search(ctx context.Context, r SearchRequest) (model.WikiSearchResult, error) {
	return model.WikiSearchResult{}, errors.New("search_wiki is not implemented")
}
