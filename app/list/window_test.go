// SPDX-License-Identifier: GPL-3.0-or-later

package list

import (
	"context"
	"fmt"
	"testing"
)

// ceilingOf is a CeilingSource with a fixed answer.
type ceilingOf struct {
	max   int
	known bool
}

func (c ceilingOf) MaxResponseItems(context.Context) (int, bool) { return c.max, c.known }

func TestCut_WindowsAndReportsNext(t *testing.T) {
	t.Parallel()
	cases := []struct {
		page, limit int
		want        string
		hasNext     bool
	}{
		{1, 2, "[1 2]", true},
		{2, 2, "[3 4]", true},
		{3, 2, "[5]", false},
		{4, 2, "[]", false},
		{9, 2, "[]", false},
	}
	for _, c := range cases {
		items, next := Cut(corpus(5), Paging{Page: c.page, Limit: c.limit})
		if fmt.Sprint(items) != c.want || next != c.hasNext {
			t.Errorf("page %d limit %d: got %v next %v, want %s next %v",
				c.page, c.limit, items, next, c.want, c.hasNext)
		}
	}
}

func TestWindow_DefaultLimitIsLoweredToAKnownCeiling(t *testing.T) {
	t.Parallel()
	var got Paging
	_, used, err := Window(context.Background(), ceilingOf{max: 3, known: true}, Paging{Page: 1}, 10,
		func(_ context.Context, p Paging) ([]int, error) {
			got = p
			return nil, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if got.Limit != 3 || used.Limit != 3 {
		t.Fatalf("fetched limit %d, reported %d, want 3 (default 10 lowered to the ceiling)", got.Limit, used.Limit)
	}
}

func TestWindow_PassesPageZeroThroughAsGiven(t *testing.T) {
	t.Parallel()
	var got Paging
	_, used, err := Window(context.Background(), ceilingOf{}, Paging{Page: 0}, 10,
		func(_ context.Context, p Paging) ([]int, error) {
			got = p
			return nil, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if got.Page != 0 || used.Page != 0 {
		t.Fatalf("page reached the fetch as %d, want 0 passed through", got.Page)
	}
}

func TestWindow_OverKnownCeilingRefuses(t *testing.T) {
	t.Parallel()
	called := false
	_, _, err := Window(context.Background(), ceilingOf{max: 3, known: true}, Paging{Page: 1, Limit: 4}, 10,
		func(context.Context, Paging) ([]int, error) {
			called = true
			return nil, nil
		})
	if _, ok := err.(*OverCeilingError); !ok {
		t.Fatalf("err = %v, want *OverCeilingError", err)
	}
	if called {
		t.Fatal("fetch ran for a refused limit")
	}
}
