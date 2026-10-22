// SPDX-License-Identifier: GPL-3.0-or-later

package list

import (
	"context"
	"fmt"
	"math"
	"testing"
)

// A page or limit large enough to wrap (page-1)*limit must still yield an
// empty page, never a slice panic and never a window that starts before 0.
func TestCut_HugePageOrLimitDoesNotOverflow(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		page, limit int
		want        string
	}{
		{"wrapping product", 1<<61 + 1, 4, "[]"},
		{"wrapping product, limit 8", 1<<61 + 1, 8, "[]"},
		{"spec example, no wrap", 1<<61 + 1, 2, "[]"},
		{"huge page", math.MaxInt, 1, "[]"},
		{"huge limit on page 2", 2, math.MaxInt, "[]"},
		{"huge limit on page 1", 1, math.MaxInt, "[1 2 3 4 5]"},
	}
	for _, c := range cases {
		items, next := Cut(corpus(5), Paging{Page: c.page, Limit: c.limit})
		if fmt.Sprint(items) != c.want || next {
			t.Errorf("%s: got %v next %v, want %s next false", c.name, items, next, c.want)
		}
	}
}

func TestRun_HugePageDoesNotReportNextFromAWrappedOffset(t *testing.T) {
	t.Parallel()
	total := 5
	c := Ceiling{}
	page, err := Run(context.Background(), Paging{Page: 1<<61 + 1, Limit: 4}, c,
		func(context.Context, Paging) (Batch[int], error) {
			return Batch[int]{Items: nil, Total: &total}, nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if page.HasNext {
		t.Fatalf("HasNext true past the last row of a 5-row list")
	}
}
