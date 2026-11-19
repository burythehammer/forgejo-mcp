// SPDX-License-Identifier: GPL-3.0-or-later

package resource

import (
	"net/url"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/numarg"

	"github.com/mark3labs/mcp-go/mcp"
)

// PageLimit reads the page and limit query parameters of a resource read, with
// EmbeddedListCap as the ceiling for limit. The numbers are read by the
// numeric-argument rule the list tools share (numarg.Paging): an absent,
// malformed or sub-1 page is 1; an absent, malformed or sub-1 limit is
// EmbeddedListCap; a limit above EmbeddedListCap is lowered to it.
func PageLimit(req mcp.ReadResourceRequest) (page, limit int) {
	page = 1
	limit = EmbeddedListCap
	u, err := url.Parse(req.Params.URI)
	if err != nil {
		return page, limit
	}
	q := u.Query()
	if v, err := numarg.Paging("page", q.Get("page")); err == nil && v >= 1 {
		page = v
	}
	if v, err := numarg.Paging("limit", q.Get("limit")); err == nil && v >= 1 {
		limit = min(v, EmbeddedListCap)
	}
	return page, limit
}
