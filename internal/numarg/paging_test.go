// SPDX-License-Identifier: GPL-3.0-or-later

package numarg

import (
	"math"
	"testing"
)

// TestPagingReadsEveryListArgumentTheSameWay pins the one rule every list tool
// and resource shares for a page or a limit: a whole number is itself, a
// fraction truncates, zero and anything below one mean "use the default", and
// a value above MaxPaging is clamped to it. Only a non-number is refused.
func TestPagingReadsEveryListArgumentTheSameWay(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		json    string
		want    int
		wantErr bool
	}{
		{name: "omitted", json: "<omitted>", want: 0},
		{name: "null", json: "null", want: 0},
		{name: "empty string", json: `""`, want: 0},
		{name: "zero", json: "0", want: 0},
		{name: "negative reads as absent", json: "-1", want: 0},
		{name: "fraction below one", json: "0.5", want: 0},
		{name: "fraction truncates", json: "2.5", want: 2},
		{name: "one", json: "1", want: 1},
		{name: "whole float", json: "3.0", want: 3},
		{name: "numeric string", json: `"7"`, want: 7},
		{name: "largest allowed", json: "2147483647", want: math.MaxInt32},
		{name: "one past the largest is clamped", json: "2147483648", want: math.MaxInt32},
		{name: "huge float is clamped", json: "1e300", want: math.MaxInt32},
		{name: "not a number", json: `"abc"`, wantErr: true},
		{name: "boolean", json: "true", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := Paging("page", raw(t, tc.json))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Paging(%s) accepted a non-number, want an error", tc.json)
				}
				return
			}
			if err != nil {
				t.Fatalf("Paging(%s) unexpected error: %v", tc.json, err)
			}
			if got != tc.want {
				t.Fatalf("Paging(%s) = %d, want %d", tc.json, got, tc.want)
			}
		})
	}
}

// TestPagingClampsTheSameHugeValueOnEveryCaller pins that a page or limit far
// beyond any real list is clamped, never wrapped and never refused, so a huge
// page reads as an empty page on every list tool and resource.
func TestPagingClampsTheSameHugeValueOnEveryCaller(t *testing.T) {
	t.Parallel()
	huge := raw(t, "1e18")
	for _, name := range []string{"page", "limit"} {
		got, err := Paging(name, huge)
		if err != nil || got != MaxPaging {
			t.Fatalf("Paging(%s, 1e18) = %d, %v; want %d, nil", name, got, err, MaxPaging)
		}
	}
}
