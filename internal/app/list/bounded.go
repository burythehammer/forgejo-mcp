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

// Map returns b with each row converted by f. The paging and the resume
// information are kept, so a use case can change the row type of a page
// without building a new one.
func Map[T, U any](b Bounded[T], f func(T) U) Bounded[U] {
	items := make([]U, 0, len(b.Items))
	for _, it := range b.Items {
		items = append(items, f(it))
	}
	return Bounded[U]{
		Items:   items,
		Page:    b.Page,
		Limit:   b.Limit,
		Count:   len(items),
		HasNext: b.HasNext,
		Total:   b.Total,
	}
}

// Extend returns b with more rows appended to the rows shown. Count follows
// the rows shown; the paging, HasNext and Total are those of b, because the
// appended rows are not part of the paged list (a list merged with an
// unpaged one, such as labels with an organization's labels).
func (b Bounded[T]) Extend(more ...T) Bounded[T] {
	items := make([]T, 0, len(b.Items)+len(more))
	items = append(items, b.Items...)
	items = append(items, more...)
	b.Items = items
	b.Count = len(items)
	return b
}

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
