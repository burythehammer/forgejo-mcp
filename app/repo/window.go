// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/list"
)

// ceilinged is the part of a repository port the list use cases read the
// instance's pagination ceiling from.
type ceilinged interface {
	MaxResponseItems(ctx context.Context) (limit int, ok bool)
}

// window reads one page of a repository list through the list module. The page
// is passed on as given; a limit below 1 takes def, lowered to the instance
// ceiling. It returns the paging the fetch used, so a caller can report it.
func window[T any](ctx context.Context, src ceilinged, pg list.Paging, def int,
	fetch func(context.Context, list.Paging) ([]T, error),
) ([]T, list.Paging, error) {
	ceiling, known := src.MaxResponseItems(ctx)
	c := list.Ceiling{Max: ceiling, Known: known}
	p := list.Given(pg, def, c)
	items, err := list.Items(ctx, p, c, func(ctx context.Context, q list.Paging) (list.Batch[T], error) {
		got, err := fetch(ctx, q)
		return list.Batch[T]{Items: got}, err
	})
	return items, p, err
}
