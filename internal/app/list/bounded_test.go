// SPDX-License-Identifier: GPL-3.0-or-later

package list

import (
	"context"
	"fmt"
	"testing"
)

// boundedCase is one boundary of the bounded result: the items shown, whether
// rows follow, the grand total (nil when unknown) and the count.
type boundedCase struct {
	items   string
	hasNext bool
	total   *int
	count   int
}

func checkBounded[T any](t *testing.T, got Bounded[T], want boundedCase) {
	t.Helper()
	if fmt.Sprint(got.Items) != want.items {
		t.Errorf("items = %v, want %s", got.Items, want.items)
	}
	if got.HasNext != want.hasNext {
		t.Errorf("HasNext = %v, want %v", got.HasNext, want.hasNext)
	}
	if got.Count != want.count {
		t.Errorf("Count = %d, want %d", got.Count, want.count)
	}
	switch {
	case want.total == nil && got.Total != nil:
		t.Errorf("Total = %d, want unknown (nil)", *got.Total)
	case want.total != nil && (got.Total == nil || *got.Total != *want.total):
		t.Errorf("Total = %v, want %d", got.Total, *want.total)
	}
}

func intp(n int) *int { return &n }

func TestBounded_Run_Boundaries(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		items []int
		known bool
		want  boundedCase
	}{
		{"exactly one page, known ceiling", corpus(3), true,
			boundedCase{items: "[1 2 3]", hasNext: false, count: 3}},
		{"one past the page, known ceiling", corpus(4), true,
			boundedCase{items: "[1 2 3]", hasNext: true, count: 3}},
		{"empty list", nil, true,
			boundedCase{items: "[]", hasNext: false, count: 0}},
		{"exactly one page, no total, unknown ceiling", corpus(3), false,
			boundedCase{items: "[1 2 3]", hasNext: false, count: 3}},
		{"one past, no total, unknown ceiling", corpus(4), false,
			boundedCase{items: "[1 2 3]", hasNext: true, count: 3}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, err := Run(context.Background(), Paging{Page: 1, Limit: 3},
				Ceiling{Max: 100, Known: c.known}, fetchFrom(c.items, nil))
			if err != nil {
				t.Fatal(err)
			}
			checkBounded(t, got, c.want)
			if got.Page != 1 || got.Limit != 3 {
				t.Errorf("paging = %d/%d, want 1/3", got.Page, got.Limit)
			}
		})
	}
}

func TestBounded_Run_ReportedTotalIsKept(t *testing.T) {
	t.Parallel()
	got, err := Run(context.Background(), Paging{Page: 1, Limit: 3}, Ceiling{},
		withTotal(corpus(4), 4))
	if err != nil {
		t.Fatal(err)
	}
	checkBounded(t, got, boundedCase{items: "[1 2 3]", hasNext: true, total: intp(4), count: 3})
}

func TestBounded_Window_Boundaries(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		items []int
		more  bool
		total *int
		want  boundedCase
	}{
		{"exactly one page, port says no more", corpus(3), false, nil,
			boundedCase{items: "[1 2 3]", hasNext: false, count: 3}},
		{"one past, port says more", corpus(4), true, nil,
			boundedCase{items: "[1 2 3]", hasNext: true, count: 3}},
		{"empty, port says no more", nil, false, nil,
			boundedCase{items: "[]", hasNext: false, count: 0}},
		{"one past, port reports total", corpus(4), true, intp(4),
			boundedCase{items: "[1 2 3]", hasNext: true, total: intp(4), count: 3}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			var calls int
			got, err := Window(context.Background(), ceilingOf{}, Paging{Page: 1, Limit: 3}, 10,
				func(_ context.Context, p Paging) (Batch[int], error) {
					calls++
					return Batch[int]{Items: c.items[:min(len(c.items), p.Limit)], Total: c.total, More: c.more}, nil
				})
			if err != nil {
				t.Fatal(err)
			}
			checkBounded(t, got, c.want)
			if calls != 1 {
				t.Errorf("Window made %d fetches, want 1 (no probe)", calls)
			}
		})
	}
}

func TestBounded_Cut_Boundaries(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		items []int
		page  int
		want  boundedCase
	}{
		{"exactly one page", corpus(3), 1,
			boundedCase{items: "[1 2 3]", hasNext: false, total: intp(3), count: 3}},
		{"one past the page", corpus(4), 1,
			boundedCase{items: "[1 2 3]", hasNext: true, total: intp(4), count: 3}},
		{"empty list", nil, 1,
			boundedCase{items: "[]", hasNext: false, total: intp(0), count: 0}},
		{"page past the end", corpus(4), 3,
			boundedCase{items: "[]", hasNext: false, total: intp(4), count: 0}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := Cut(c.items, Paging{Page: c.page, Limit: 3})
			checkBounded(t, got, c.want)
		})
	}
}
