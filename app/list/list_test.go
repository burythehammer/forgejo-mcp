// SPDX-License-Identifier: GPL-3.0-or-later

package list

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

// corpus is a fixed list of n items 1..n, served the way Forgejo serves a
// paged endpoint: page p at size s is the window (p-1)*s+1 .. p*s.
func corpus(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i + 1
	}
	return out
}

// fetchFrom returns a Fetch over items that also records every page it was asked for.
func fetchFrom(items []int, calls *[]Paging) Fetch[int] {
	return func(_ context.Context, p Paging) (Batch[int], error) {
		if calls != nil {
			*calls = append(*calls, p)
		}
		lo := (p.Page - 1) * p.Limit
		if lo > len(items) {
			lo = len(items)
		}
		hi := lo + p.Limit
		if hi > len(items) {
			hi = len(items)
		}
		return Batch[int]{Items: items[lo:hi]}, nil
	}
}

func TestNormalize_DefaultsAndClamps(t *testing.T) {
	cases := []struct {
		name string
		in   Paging
		want Paging
	}{
		{"zero is default page and limit", Paging{}, Paging{Page: 1, Limit: 20}},
		{"explicit kept", Paging{Page: 3, Limit: 7}, Paging{Page: 3, Limit: 7}},
		{"negative page clamps to first", Paging{Page: -4, Limit: 5}, Paging{Page: 1, Limit: 5}},
		{"negative limit takes default", Paging{Page: 2, Limit: -1}, Paging{Page: 2, Limit: 20}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Normalize(c.in, 20); got != c.want {
				t.Fatalf("Normalize(%+v) = %+v, want %+v", c.in, got, c.want)
			}
		})
	}
}

func TestRun_DefaultAndExplicitPage(t *testing.T) {
	items := corpus(5)
	ctx := context.Background()

	page, err := Run(ctx, Paging{Page: 1, Limit: 2}, Ceiling{}, fetchFrom(items, nil))
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(page.Items) != "[1 2]" || page.Page != 1 || page.Limit != 2 || page.Count != 2 || !page.HasNext {
		t.Fatalf("page 1 = %+v", page)
	}

	page, err = Run(ctx, Paging{Page: 2, Limit: 2}, Ceiling{}, fetchFrom(items, nil))
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(page.Items) != "[3 4]" || page.Count != 2 || !page.HasNext {
		t.Fatalf("page 2 = %+v", page)
	}
}

func TestRun_LastPageHasNoNext(t *testing.T) {
	items := corpus(5)
	page, err := Run(context.Background(), Paging{Page: 3, Limit: 2}, Ceiling{Max: 100, Known: true}, fetchFrom(items, nil))
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(page.Items) != "[5]" || page.Count != 1 || page.HasNext {
		t.Fatalf("last page = %+v", page)
	}
}

func TestRun_PastTheEndIsEmptyAndHasNoNext(t *testing.T) {
	page, err := Run(context.Background(), Paging{Page: 9, Limit: 2}, Ceiling{}, fetchFrom(corpus(5), nil))
	if err != nil {
		t.Fatal(err)
	}
	if page.Count != 0 || page.HasNext {
		t.Fatalf("past end = %+v", page)
	}
}

func TestRun_KnownCeilingFullPageProbesOnce(t *testing.T) {
	// A full page could be the last one: an exact multiple of the limit. The
	// next page is probed once at the same size, so has_next is exact.
	var calls []Paging
	page, err := Run(context.Background(), Paging{Page: 1, Limit: 2}, Ceiling{Max: 10, Known: true}, fetchFrom(corpus(4), &calls))
	if err != nil {
		t.Fatal(err)
	}
	if !page.HasNext {
		t.Fatalf("full page with more rows should report next: %+v", page)
	}
	if want := []Paging{{Page: 1, Limit: 2}, {Page: 2, Limit: 2}}; fmt.Sprint(calls) != fmt.Sprint(want) {
		t.Fatalf("fetches = %v, want %v", calls, want)
	}
}

func TestRun_KnownCeilingExactMultipleLastPageHasNoNext(t *testing.T) {
	// The last page is full, so the probe finds nothing and has_next is false.
	var calls []Paging
	page, err := Run(context.Background(), Paging{Page: 2, Limit: 2}, Ceiling{Max: 10, Known: true}, fetchFrom(corpus(4), &calls))
	if err != nil {
		t.Fatal(err)
	}
	if page.HasNext {
		t.Fatalf("exact-multiple last page should not report next: %+v", page)
	}
	if len(calls) != 2 {
		t.Fatalf("fetches = %v, want the page and one probe", calls)
	}
}

func TestRun_KnownCeilingShortPageMakesNoProbe(t *testing.T) {
	// A page shorter than the limit is the end under a known ceiling: no probe.
	var calls []Paging
	page, err := Run(context.Background(), Paging{Page: 3, Limit: 2}, Ceiling{Max: 10, Known: true}, fetchFrom(corpus(5), &calls))
	if err != nil {
		t.Fatal(err)
	}
	if page.HasNext || page.Count != 1 {
		t.Fatalf("short last page: %+v", page)
	}
	if len(calls) != 1 {
		t.Fatalf("fetches = %v, want the page only", calls)
	}
}

func TestRun_UnknownCeilingProbesNextPageAtSameSize(t *testing.T) {
	var calls []Paging
	page, err := Run(context.Background(), Paging{Page: 1, Limit: 2}, Ceiling{}, fetchFrom(corpus(5), &calls))
	if err != nil {
		t.Fatal(err)
	}
	if !page.HasNext {
		t.Fatalf("unknown ceiling with more rows should report next: %+v", page)
	}
	// The probe keeps the same limit: a changed size would skip rows.
	want := []Paging{{Page: 1, Limit: 2}, {Page: 2, Limit: 2}}
	if fmt.Sprint(calls) != fmt.Sprint(want) {
		t.Fatalf("fetches = %v, want %v", calls, want)
	}
}

func TestRun_UnknownCeilingLastPageHasNoNext(t *testing.T) {
	page, err := Run(context.Background(), Paging{Page: 3, Limit: 2}, Ceiling{}, fetchFrom(corpus(5), nil))
	if err != nil {
		t.Fatal(err)
	}
	if page.HasNext {
		t.Fatalf("last page with unknown ceiling should not report next: %+v", page)
	}
}

func TestRun_OverKnownCeilingRejectsBeforeFetching(t *testing.T) {
	var calls []Paging
	_, err := Run(context.Background(), Paging{Page: 1, Limit: 3}, Ceiling{Max: 2, Known: true}, fetchFrom(corpus(5), &calls))
	var over *OverCeilingError
	if !errors.As(err, &over) {
		t.Fatalf("err = %v, want *OverCeilingError", err)
	}
	if over.Limit != 3 || over.Max != 2 {
		t.Fatalf("over = %+v", over)
	}
	want := "limit 3 exceeds this instance's maximum of 2 (max_response_items); retry with limit <= 2"
	if err.Error() != want {
		t.Fatalf("message = %q, want %q", err.Error(), want)
	}
	if len(calls) != 0 {
		t.Fatalf("fetched %d pages past the ceiling, want none", len(calls))
	}
}

func TestRun_UnknownCeilingNeverRejects(t *testing.T) {
	// An unknown ceiling is not a ceiling of zero.
	page, err := Run(context.Background(), Paging{Page: 1, Limit: 50}, Ceiling{Max: 0, Known: false}, fetchFrom(corpus(5), nil))
	if err != nil {
		t.Fatalf("unknown ceiling rejected a limit: %v", err)
	}
	if page.Count != 5 {
		t.Fatalf("count = %d, want 5", page.Count)
	}
}

func TestRun_PassesTotalThrough(t *testing.T) {
	fetch := func(context.Context, Paging) (Batch[int], error) {
		return Batch[int]{Items: []int{1}, Total: Total{Count: 42, Known: true}}, nil
	}
	page, err := Run(context.Background(), Paging{Page: 1, Limit: 1}, Ceiling{Max: 10, Known: true}, fetch)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != (Total{Count: 42, Known: true}) {
		t.Fatalf("total = %+v", page.Total)
	}
}

// withTotal serves items like fetchFrom and reports total as the grand total.
func withTotal(items []int, total int) Fetch[int] {
	inner := fetchFrom(items, nil)
	return func(ctx context.Context, p Paging) (Batch[int], error) {
		b, err := inner(ctx, p)
		b.Total = Total{Count: total, Known: true}
		return b, err
	}
}

func TestRun_KnownTotalDecidesHasNextWithoutProbe(t *testing.T) {
	// A reported grand total is authoritative: has_next is whether rows past
	// this page exist, and no probe is made.
	cases := []struct {
		name    string
		page    Paging
		total   int
		hasNext bool
	}{
		{"first page of more", Paging{Page: 1, Limit: 2}, 42, true},
		{"last page short of total", Paging{Page: 21, Limit: 2}, 42, false},
		{"exact multiple last page", Paging{Page: 2, Limit: 2}, 4, false},
		{"full page before exact end", Paging{Page: 1, Limit: 2}, 4, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var calls []Paging
			fetch := func(ctx context.Context, p Paging) (Batch[int], error) {
				calls = append(calls, p)
				return withTotal(corpus(tc.total), tc.total)(ctx, p)
			}
			page, err := Run(context.Background(), tc.page, Ceiling{Max: 100, Known: true}, fetch)
			if err != nil {
				t.Fatal(err)
			}
			if page.HasNext != tc.hasNext {
				t.Fatalf("has_next = %v, want %v: %+v", page.HasNext, tc.hasNext, page)
			}
			if len(calls) != 1 {
				t.Fatalf("known total made %d fetches, want 1: %v", len(calls), calls)
			}
		})
	}
}

func TestRun_FetchErrorSurfaces(t *testing.T) {
	boom := errors.New("upstream down")
	fetch := func(context.Context, Paging) (Batch[int], error) { return Batch[int]{}, boom }
	if _, err := Run(context.Background(), Paging{Page: 1, Limit: 2}, Ceiling{}, fetch); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want upstream error", err)
	}
}

func TestItems_ReturnsOnlyTheWindowWithoutProbing(t *testing.T) {
	var calls []Paging
	items, err := Items(context.Background(), Paging{Page: 1, Limit: 2}, Ceiling{}, fetchFrom(corpus(5), &calls))
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(items) != "[1 2]" {
		t.Fatalf("items = %v", items)
	}
	if len(calls) != 1 {
		t.Fatalf("Items made %d fetches, want 1 (no next probe)", len(calls))
	}
}

func TestItems_OverKnownCeilingRejects(t *testing.T) {
	_, err := Items(context.Background(), Paging{Page: 1, Limit: 9}, Ceiling{Max: 4, Known: true}, fetchFrom(corpus(5), nil))
	var over *OverCeilingError
	if !errors.As(err, &over) {
		t.Fatalf("err = %v, want *OverCeilingError", err)
	}
}

// DefaultLimit lowers a tool's default to a known ceiling, so a default call
// never trips the ceiling, while an unknown or larger ceiling leaves it alone.
func TestDefaultLimit_ClampsToKnownCeiling(t *testing.T) {
	cases := []struct {
		name string
		def  int
		c    Ceiling
		want int
	}{
		{"known ceiling below default", 30, Ceiling{Max: 10, Known: true}, 10},
		{"known ceiling above default", 30, Ceiling{Max: 50, Known: true}, 30},
		{"unknown ceiling", 30, Ceiling{Max: 0, Known: false}, 30},
		{"known ceiling equal to default", 30, Ceiling{Max: 30, Known: true}, 30},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DefaultLimit(tc.def, tc.c); got != tc.want {
				t.Fatalf("DefaultLimit(%d, %+v) = %d, want %d", tc.def, tc.c, got, tc.want)
			}
		})
	}
}

// Given keeps the page exactly as the caller gave it, because some tools pin
// page 0 reaching the server, and takes a limit below 1 as the default lowered
// to a known ceiling.
func TestGiven_KeepsPageAndDefaultsLimit(t *testing.T) {
	cases := []struct {
		name string
		p    Paging
		c    Ceiling
		want Paging
	}{
		{"page 0 is kept", Paging{Page: 0, Limit: 5}, Ceiling{}, Paging{Page: 0, Limit: 5}},
		{"absent limit takes default", Paging{Page: 2}, Ceiling{}, Paging{Page: 2, Limit: 100}},
		{"absent limit lowered to known ceiling", Paging{Page: 1}, Ceiling{Max: 10, Known: true}, Paging{Page: 1, Limit: 10}},
		{"explicit limit is not defaulted", Paging{Page: 1, Limit: 7}, Ceiling{Max: 10, Known: true}, Paging{Page: 1, Limit: 7}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Given(tc.p, 100, tc.c); got != tc.want {
				t.Fatalf("Given(%+v) = %+v, want %+v", tc.p, got, tc.want)
			}
		})
	}
}
