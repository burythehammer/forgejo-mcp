// SPDX-License-Identifier: GPL-3.0-or-later

// Package list is the one place a list use case pages, bounds and reports
// what it returned. A use case hands it a page request, the instance's
// pagination ceiling and a fetch function; it gets back the page with a next
// marker. It is MCP-free and SDK-free like the rest of internal/app/: the ceiling is an
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

// Request is what the agent asked for plus the tool's default limit. The list
// module sees the raw request so it can report when the limit it applied
// differs from the one asked for or defaulted to. Page and Limit are as sent
// (zero when omitted); Def is the tool's default limit.
type Request struct{ Page, Limit, Def int }

// NewRequest is the Request for a raw paging and the tool's default limit.
func NewRequest(p Paging, def int) Request {
	return Request{Page: p.Page, Limit: p.Limit, Def: def}
}

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
// representation of an unknown total in internal/app/list and internal/app/model alike.
type Batch[T any] struct {
	Items []T
	Total *int
	// More is the port's own report that rows follow the page. Window reads it;
	// Run decides from Total or probes and ignores it.
	More bool
}

// Reported is the grand total of a list when the server reported one, nil when
// it did not. An unknown total is nil, never zero.
func Reported(count int, known bool) *int {
	if !known {
		return nil
	}
	return &count
}

// Fetch reads one page of a list. It is called with a normalized Paging.
type Fetch[T any] func(ctx context.Context, p Paging) (Batch[T], error)

// Run fetches one page and reports whether a next page exists. A known
// ceiling is checked before any fetch. When the fetch reports a grand total,
// that total decides and no probe is made. Otherwise a page shorter than the
// limit is the last one under a known ceiling, so no probe is made; in every
// other case the next page is probed once at the same limit: a full page may
// be the last, and the size never changes, because Forgejo derives page
// offsets from it. So a call makes at most one request beyond its page.
func Run[T any](ctx context.Context, req Request, c Ceiling, fetch Fetch[T]) (Bounded[T], error) {
	p := Given(Paging{Page: req.Page, Limit: req.Limit}, req.Def, c)
	if err := check(p, c); err != nil {
		return Bounded[T]{}, err
	}
	b, err := fetch(ctx, p)
	if err != nil {
		return Bounded[T]{}, err
	}
	count := len(b.Items)
	var hasNext bool
	if b.Total != nil {
		// A reported grand total decides: rows follow this page when the rows
		// before it plus this page fall short of the total. No probe is made.
		hasNext = Start(p, *b.Total)+count < *b.Total
	} else if !c.Known || count >= p.Limit {
		next, err := fetch(ctx, Paging{Page: p.Page + 1, Limit: p.Limit})
		if err != nil {
			return Bounded[T]{}, err
		}
		hasNext = len(next.Items) > 0
	}
	return newBounded(b.Items, p, req.requested(), b.Total, hasNext), nil
}

func check(p Paging, c Ceiling) error {
	if c.Known && p.Limit > c.Max {
		return &OverCeilingError{Limit: p.Limit, Max: c.Max}
	}
	return nil
}

// requested is the limit the caller asked for, or the default when none was
// given. It is what newBounded compares the applied limit against.
func (r Request) requested() int {
	if r.Limit >= 1 {
		return r.Limit
	}
	return r.Def
}
