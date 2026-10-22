// SPDX-License-Identifier: GPL-3.0-or-later

package list

// Bounded is the one result a list use case returns for bounded output: the
// items shown, the grand total (nil when unknown, never zero) and how to
// continue. Run, Window and Cut are the only operations that produce it, through
// newBounded, so no caller can build one from a bare slice.
type Bounded[T any] struct {
	// Items are the rows shown in this page.
	Items []T
	// Page and Limit are the paging the rows were read with.
	Page  int
	Limit int
	// Count is the number of rows in Items.
	Count int
	// HasNext reports that more rows follow this page.
	HasNext bool
	// Total is the grand total of the list, nil when the server reported none.
	Total *int
}

// Page is the former name of Bounded. It stays an alias so existing callers
// compile until they migrate to Bounded.
type Page[T any] = Bounded[T]

// newBounded is the only constructor of Bounded. It is unexported so that a
// use case cannot bypass Run, Window or Cut.
func newBounded[T any](items []T, p Paging, total *int, hasNext bool) Bounded[T] {
	return Bounded[T]{
		Items:   items,
		Page:    p.Page,
		Limit:   p.Limit,
		Count:   len(items),
		HasNext: hasNext,
		Total:   total,
	}
}
