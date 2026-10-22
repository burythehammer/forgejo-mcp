// SPDX-License-Identifier: GPL-3.0-or-later

package list

import "context"

// Everything is every row of a list, read page by page through the module. It
// is for a derived value that needs all rows, such as a combined state. It is
// not a page a caller can show as the whole list.
type Everything[T any] struct {
	// Items are the rows read, in list order.
	Items []T
	// Total is the grand total the server reported, nil when it reported none.
	Total *int
	// Complete is false when the page budget stopped the read before the last
	// row. A derived value must not be computed from an incomplete read.
	Complete bool
}

// All reads every page of a list at limit, and at most maxPages pages. When the
// fetch reports a grand total, the read ends once that many rows are in hand,
// and an empty page before it ends the read as incomplete. Without a total, a
// page shorter than limit is the last. A read the budget stops is Complete
// false. limit and maxPages are positive constants chosen by the caller.
func All[T any](ctx context.Context, limit, maxPages int, fetch Fetch[T]) (Everything[T], error) {
	var out Everything[T]
	for page := 1; page <= maxPages; page++ {
		b, err := fetch(ctx, Paging{Page: page, Limit: limit})
		if err != nil {
			return Everything[T]{}, err
		}
		out.Items = append(out.Items, b.Items...)
		if b.Total != nil {
			out.Total = b.Total
		}
		switch {
		case out.Total != nil:
			if len(out.Items) >= *out.Total {
				out.Complete = true
				return out, nil
			}
			if len(b.Items) == 0 {
				return out, nil
			}
		case len(b.Items) < limit:
			out.Complete = true
			return out, nil
		}
	}
	return out, nil
}
