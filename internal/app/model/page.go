// SPDX-License-Identifier: GPL-3.0-or-later

package model

import "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/list"

// PageWindow is the window of a paged list response: the page asked for, the
// limit it was read with and how many rows it holds.
type PageWindow struct {
	Page  int `json:"page"`
	Limit int `json:"limit"`
	Count int `json:"count"`
}

// PageNext is the continuation signal of a paged list response.
type PageNext struct {
	HasNext bool `json:"has_next"`
}

// PageTotal is the grand total of a paged list, omitted when the server did not
// report one. It is nil, never zero, when unknown.
type PageTotal struct {
	TotalCount *int `json:"total_count,omitempty"`
}

// PageMeta is the continuation metadata of a paged list response, the wire form
// of docs/design/output-bounding.md, in the common key order: page, limit,
// count, has_next, total_count. A page type embeds it last. encoding/json
// flattens embedded fields in place, so its own fields come first.
//
// A page whose existing wire order puts total_count before has_next embeds
// PageWindow, PageTotal and PageNext in that order instead.
type PageMeta struct {
	PageWindow
	PageNext
	PageTotal
}

// PageMetaOf is the one place a list.Page becomes its response metadata.
func PageMetaOf[T any](p list.Page[T]) PageMeta {
	return PageMeta{
		PageWindow: PageWindow{Page: p.Page, Limit: p.Limit, Count: p.Count},
		PageNext:   PageNext{HasNext: p.HasNext},
		PageTotal:  PageTotal{TotalCount: p.Total},
	}
}
