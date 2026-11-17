// SPDX-License-Identifier: GPL-3.0-or-later

package list

import (
	"context"
	"errors"
	"fmt"
)

// FileCeiling is the most rows a whole-listing file output may hold: a listing
// of exactly this many rows is written whole, and one more row fails. It is a
// separate, explicit limit, not the instance ceiling (max_response_items) that
// lowers a page's limit, and it does not weaken the per-page bound. Every list
// that writes a whole listing to a file shares it. See GLOSSARY.md, "Ceiling".
const FileCeiling = 1000

// ErrShortRead is returned when a listing ends before the grand total the
// server reported: the read is incomplete, so no rows are returned.
var ErrShortRead = errors.New("listing ended before the total the server reported; nothing was written")

// OverCeiling is returned when a whole listing holds more than Max rows. It
// carries the ceiling so the caller can name it.
type OverCeiling struct{ Max int }

func (e *OverCeiling) Error() string {
	return fmt.Sprintf("listing is over the %d-row ceiling; nothing was written", e.Max)
}

// ReadWhole reads every row of a list page by page at limit and returns them in
// list order. It stops as soon as more than ceiling rows are in hand, or as soon
// as the server's reported total is over ceiling, and returns *OverCeiling: a
// listing is never truncated to fit. Without a reported total, a page shorter
// than limit is the last. Its shortfall rule (ErrShortRead) differs from All's
// (Complete false), and its early stop on size is not All's page budget, so it
// keeps its own loop.
func ReadWhole[T any](ctx context.Context, limit, ceiling int, fetch Fetch[T]) ([]T, error) {
	var out []T
	for page := 1; ; page++ {
		b, err := fetch(ctx, Paging{Page: page, Limit: limit})
		if err != nil {
			return nil, err
		}
		if b.Total != nil && *b.Total > ceiling {
			return nil, &OverCeiling{Max: ceiling}
		}
		out = append(out, b.Items...)
		if len(out) > ceiling {
			return nil, &OverCeiling{Max: ceiling}
		}
		if b.Total != nil {
			if len(out) >= *b.Total {
				return out, nil
			}
			if len(b.Items) == 0 {
				return nil, ErrShortRead
			}
			continue
		}
		if len(b.Items) < limit {
			return out, nil
		}
	}
}
