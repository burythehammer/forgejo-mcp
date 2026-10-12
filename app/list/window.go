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
	p := Given(pg, def, c)
	items, err := Items(ctx, p, c, func(ctx context.Context, q Paging) (Batch[T], error) {
		got, err := fetch(ctx, q)
		return Batch[T]{Items: got}, err
	})
	return items, p, err
}

// Cut returns the rows of all that page p names, and whether rows follow them.
// It is for a list Forgejo does not page: the whole list is fetched, then cut
// here. A page past the end is empty and has no next page.
func Cut[T any](all []T, p Paging) ([]T, bool) {
	start64 := int64(p.Page-1) * int64(p.Limit)
	start := len(all)
	if start64 < int64(len(all)) {
		start = int(start64)
	}
	end := min(start+p.Limit, len(all))
	return all[start:end], end < len(all)
}
