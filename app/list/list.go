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

// Total is a count a list reports. Known is false when the server did not
// report one, in which case Count means nothing.
type Total struct {
	Count int
	Known bool
}

// Batch is what one fetch of a page returns.
type Batch[T any] struct {
	Items []T
	Total Total
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
	Total   Total
}

// Run fetches one page and reports whether a next page exists. A known
// ceiling is checked before any fetch, and a full page then means more may
// follow. With an unknown ceiling the next page is probed at the same limit:
// the size never changes, because Forgejo derives page offsets from it.
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
	if c.Known {
		hasNext = count == p.Limit
	} else if count > 0 {
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

// Items fetches one page for a list whose envelope carries no continuation
// marker. It applies the ceiling and makes no probe request.
func Items[T any](ctx context.Context, p Paging, c Ceiling, fetch Fetch[T]) ([]T, error) {
	if err := check(p, c); err != nil {
		return nil, err
	}
	b, err := fetch(ctx, p)
	if err != nil {
		return nil, err
	}
	return b.Items, nil
}

func check(p Paging, c Ceiling) error {
	if c.Known && p.Limit > c.Max {
		return &OverCeilingError{Limit: p.Limit, Max: c.Max}
	}
	return nil
}
