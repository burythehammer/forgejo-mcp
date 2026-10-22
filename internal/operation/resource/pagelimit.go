// SPDX-License-Identifier: GPL-3.0-or-later

package resource

import (
	"net/url"
	"strconv"

	"github.com/mark3labs/mcp-go/mcp"
)

// PageLimit reads the page and limit query parameters of a resource read, with
// EmbeddedListCap as the ceiling for limit. An absent or malformed page is 1;
// an absent or malformed limit is EmbeddedListCap, and a limit above it is
// lowered to it.
func PageLimit(req mcp.ReadResourceRequest) (page, limit int) {
	page = 1
	limit = EmbeddedListCap
	u, err := url.Parse(req.Params.URI)
	if err != nil {
		return page, limit
	}
	q := u.Query()
	if v, err := strconv.Atoi(q.Get("page")); err == nil && v >= 1 {
		page = v
	}
	if v, err := strconv.Atoi(q.Get("limit")); err == nil && v >= 1 {
		limit = min(v, EmbeddedListCap)
	}
	return page, limit
}
