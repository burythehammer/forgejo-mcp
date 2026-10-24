// SPDX-License-Identifier: GPL-3.0-or-later

package decode

import (
	"math"
	"testing"
)

// TestPagingArgsMatchesTheListRule pins the list rule for the two arguments
// every list tool shares: an absent or zero page is 1, an absent or zero limit
// is 0 (the tool default), a fraction truncates, a negative value reads as
// absent, and a value past numarg.MaxPaging is clamped to it. None is an error.
func TestPagingArgsMatchesTheListRule(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		page, limit float64
		wantPage    int
		wantL       int
	}{
		{name: "absent", wantPage: 1, wantL: 0},
		{name: "explicit zero", page: 0, limit: 0, wantPage: 1, wantL: 0},
		{name: "page and limit", page: 3, limit: 50, wantPage: 3, wantL: 50},
		{name: "largest allowed", page: math.MaxInt32, limit: math.MaxInt32, wantPage: math.MaxInt32, wantL: math.MaxInt32},
		{name: "page fraction truncates", page: 2.5, wantPage: 2, wantL: 0},
		{name: "limit fraction below one is absent", limit: 0.5, wantPage: 1, wantL: 0},
		{name: "negative page reads as page one", page: -2, wantPage: 1, wantL: 0},
		{name: "huge page is clamped", page: 1e18, wantPage: math.MaxInt32, wantL: 0},
		{name: "huge limit is clamped", limit: 1e18, wantPage: 1, wantL: math.MaxInt32},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p, l, err := PagingArgs(tc.page, tc.limit)
			if err != nil {
				t.Fatalf("PagingArgs(%v, %v) unexpected error: %v", tc.page, tc.limit, err)
			}
			if p != tc.wantPage || l != tc.wantL {
				t.Fatalf("PagingArgs(%v, %v) = %d, %d; want %d, %d", tc.page, tc.limit, p, l, tc.wantPage, tc.wantL)
			}
		})
	}
}
