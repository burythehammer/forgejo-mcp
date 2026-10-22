// SPDX-License-Identifier: GPL-3.0-or-later

package list

import "context"

// FetchPage reads one page of a list through the module: the instance ceiling
// is read from src per call, a default limit def is lowered to a known ceiling,
// and a page below 1 is page 1. The continuation marker comes from Run.
func FetchPage[T any](ctx context.Context, src CeilingSource, req Request,
	fetch func(context.Context, Paging) ([]T, error),
) (Bounded[T], error) {
	max, known := src.MaxResponseItems(ctx)
	c := Ceiling{Max: max, Known: known}
	return Run(ctx, req, c,
		func(ctx context.Context, q Paging) (Batch[T], error) {
			items, err := fetch(ctx, q)
			return Batch[T]{Items: items}, err
		})
}
