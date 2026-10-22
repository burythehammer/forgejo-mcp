// SPDX-License-Identifier: GPL-3.0-or-later

package list

import "context"

// CeilingSource is a port that reports the instance's pagination ceiling. Every
// list use case's port has it, so a use case passes its port straight in.
type CeilingSource interface {
	MaxResponseItems(ctx context.Context) (limit int, ok bool)
}

// Window reads one page of a list through the ceiling of src. The page is
// passed on as given, so a caller that needs page 0 to reach the server keeps
// it; a limit below 1 takes def, lowered to a known ceiling. It returns the
// paging the fetch used, so the caller can report it.
func Window[T any](ctx context.Context, src CeilingSource, pg Paging, def int,
	fetch func(context.Context, Paging) ([]T, error),
) ([]T, Paging, error) {
	ceiling, known := src.MaxResponseItems(ctx)
	c := Ceiling{Max: ceiling, Known: known}
	p := pg
	if p.Limit < 1 {
		p.Limit = DefaultLimit(def, c)
	}
	if err := check(p, c); err != nil {
		return nil, p, err
	}
	items, err := fetch(ctx, p)
	return items, p, err
}

// Cut returns the rows of all that page p names, and whether rows follow them.
// It is for a list Forgejo does not page: the whole list is fetched, then cut
// here. A page past the end is empty and has no next page.
func Cut[T any](all []T, p Paging) ([]T, bool) {
	start := Start(p, len(all))
	end := start
	if p.Limit > 0 {
		end = len(all)
		if p.Limit < len(all)-start {
			end = start + p.Limit
		}
	}
	return all[start:end], end < len(all)
}

// Start is the index of the first row of page p in a list of n rows, clamped
// to n: a page past the end starts at n. It never multiplies past n, so no
// page or limit, however large, can overflow into a negative offset. A page
// below 2 starts at 0.
func Start(p Paging, n int) int {
	if p.Page <= 1 || p.Limit < 1 {
		return 0
	}
	if p.Page-1 > n/p.Limit {
		return n
	}
	return (p.Page - 1) * p.Limit
}
