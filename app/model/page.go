// SPDX-License-Identifier: GPL-3.0-or-later

package model

import "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/list"

// PageMeta is the continuation metadata of a paged list response, the wire
// form of docs/design/output-bounding.md. A page type embeds it last so that
// encoding/json writes its own fields first, then page, limit, count, has_next
// and total_count (omitted when nil, meaning the server reported no total).
type PageMeta struct {
	Page       int  `json:"page"`
	Limit      int  `json:"limit"`
	Count      int  `json:"count"`
	HasNext    bool `json:"has_next"`
	TotalCount *int `json:"total_count,omitempty"`
}

// PageMetaOf is the one place a list.Page becomes its response metadata.
func PageMetaOf[T any](p list.Page[T]) PageMeta {
	return PageMeta{
		Page: p.Page, Limit: p.Limit, Count: p.Count,
		HasNext: p.HasNext, TotalCount: p.Total,
	}
}
