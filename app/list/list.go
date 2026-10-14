// SPDX-License-Identifier: GPL-3.0-or-later

// Package list is the one place a list use case pages, bounds and reports
// what it returned. A use case hands it a page request, the instance's
// pagination ceiling and a fetch function; it gets back the page with a next
// marker. It is MCP-free and SDK-free like the rest of app/: the ceiling is an
// argument the caller reads from its port per call, never a package-level
// cache.
package list

import (
	"context"
	"fmt"
)

// Paging is one page request, in the 1-based page and item limit that every
// list tool names.
type Paging struct{ Page, Limit int }

// Normalize applies the shared defaults: a page below 1 is page 1, and a limit
// below 1 is def. Callers pass the tool's default limit as def.
func Normalize(p Paging, def int) Paging {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.Limit < 1 {
		p.Limit = def
	}
	return p
}

// Ceiling is the instance's pagination ceiling. Known is false when it could
// not be determined; that is "unknown", never a ceiling of zero.
type Ceiling struct {
	Max   int
	Known bool
}

// Given is the paging a list that calls Run fetches with. A page below 1 is page
// 1, so no offset is ever computed from a page of 0 or less. A limit below 1
// takes def, lowered to a known ceiling. A caller that must pass page 0 through
// to the server uses Window, which keeps it.
func Given(p Paging, def int, c Ceiling) Paging {
	if p.Page < 1 {
		p.Page = 1
	}
	if p.Limit < 1 {
		p.Limit = DefaultLimit(def, c)
	}
	return p
}

// DefaultLimit is a tool's default limit, lowered to a known ceiling when the
// default is larger. A default is never refused by the ceiling: only a limit
// the caller asked for is.
func DefaultLimit(def int, c Ceiling) int {
	if c.Known && def > c.Max {
		return c.Max
	}
	return def
}

// OverCeilingError reports a limit above a known ceiling. The request is
// refused rather than clamped, so the caller never receives fewer items than
// it asked for without being told.
type OverCeilingError struct {
	Limit, Max int
}

func (e *OverCeilingError) Error() string {
	return fmt.Sprintf(
		"limit %d exceeds this instance's maximum of %d (max_response_items); retry with limit <= %d",
		e.Limit, e.Max, e.Max,
	)
}

// Batch is what one fetch of a page returns. Total is the server's grand total
// of the list; it is nil when the server did not report one, which is the one
// representation of an unknown total in app/list and app/model alike.
type Batch[T any] struct {
	Items []T
	Total *int
}

// Fetch reads one page of a list. It is called with a normalized Paging.
type Fetch[T any] func(ctx context.Context, p Paging) (Batch[T], error)

// Page is one bounded page with its continuation marker.
type Page[T any] struct {
	Items []T
	Page  int
	Limit int
	Count int
	// HasNext reports that more rows follow this page.
	HasNext bool
	// Total is the grand total the fetch reported, nil when it reported none.
	Total *int
}

// Run fetches one page and reports whether a next page exists. A known
// ceiling is checked before any fetch. When the fetch reports a grand total,
// that total decides and no probe is made. Otherwise a page shorter than the
// limit is the last one under a known ceiling, so no probe is made; in every
// other case the next page is probed once at the same limit: a full page may
// be the last, and the size never changes, because Forgejo derives page
// offsets from it. So a call makes at most one request beyond its page.
func Run[T any](ctx context.Context, p Paging, c Ceiling, fetch Fetch[T]) (Page[T], error) {
	if err := check(p, c); err != nil {
		return Page[T]{}, err
	}
	b, err := fetch(ctx, p)
	if err != nil {
		return Page[T]{}, err
	}
	count := len(b.Items)
	var hasNext bool
	if b.Total != nil {
		// A reported grand total decides: rows follow this page when the rows
		// before it plus this page fall short of the total. No probe is made.
		before := int64(p.Page-1) * int64(p.Limit)
		hasNext = before+int64(count) < int64(*b.Total)
	} else if !c.Known || count >= p.Limit {
		next, err := fetch(ctx, Paging{Page: p.Page + 1, Limit: p.Limit})
		if err != nil {
			return Page[T]{}, err
		}
		hasNext = len(next.Items) > 0
	}
	return Page[T]{
		Items: b.Items, Page: p.Page, Limit: p.Limit, Count: count,
		HasNext: hasNext, Total: b.Total,
	}, nil
}

func check(p Paging, c Ceiling) error {
	if c.Known && p.Limit > c.Max {
		return &OverCeilingError{Limit: p.Limit, Max: c.Max}
	}
	return nil
}
