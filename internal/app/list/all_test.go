// SPDX-License-Identifier: GPL-3.0-or-later

package list

import (
	"context"
	"errors"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/ptr"
)

func TestAll_ReadsEveryPageWithoutTotal(t *testing.T) {
	t.Parallel()
	var calls []Paging
	got, err := All(context.Background(), 3, 10, fetchFrom(corpus(7), &calls))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 7 || got.Items[0] != 1 || got.Items[6] != 7 {
		t.Fatalf("items = %v, want 1..7", got.Items)
	}
	if !got.Complete || got.Total != nil {
		t.Fatalf("complete %v total %v, want complete with no total", got.Complete, got.Total)
	}
	want := []Paging{{Page: 1, Limit: 3}, {Page: 2, Limit: 3}, {Page: 3, Limit: 3}}
	if len(calls) != len(want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
	for i := range want {
		if calls[i] != want[i] {
			t.Fatalf("call %d = %v, want %v", i, calls[i], want[i])
		}
	}
}

func TestAll_FullPageAtTheEndReadsOneEmptyPageToStop(t *testing.T) {
	t.Parallel()
	var calls []Paging
	got, err := All(context.Background(), 3, 10, fetchFrom(corpus(6), &calls))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 6 || !got.Complete {
		t.Fatalf("items %v complete %v, want 6 and complete", got.Items, got.Complete)
	}
	if len(calls) != 3 {
		t.Fatalf("calls = %v, want 3 (the third is the empty probe)", calls)
	}
}

func TestAll_EmptyListIsComplete(t *testing.T) {
	t.Parallel()
	got, err := All(context.Background(), 5, 10, fetchFrom(corpus(0), nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 0 || !got.Complete {
		t.Fatalf("empty list: items %v complete %v", got.Items, got.Complete)
	}
}

func TestAll_ReportedTotalEndsTheReadWithoutAnEmptyProbe(t *testing.T) {
	t.Parallel()
	var calls []Paging
	fetch := func(ctx context.Context, p Paging) (Batch[int], error) {
		calls = append(calls, p)
		b, err := fetchFrom(corpus(7), nil)(ctx, p)
		b.Total = ptr.To(7)
		return b, err
	}
	got, err := All(context.Background(), 3, 10, fetch)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 7 || !got.Complete || got.Total == nil || *got.Total != 7 {
		t.Fatalf("items %d complete %v total %v, want 7, complete, 7", len(got.Items), got.Complete, got.Total)
	}
	if len(calls) != 3 {
		t.Fatalf("calls = %d, want 3: the reported total must end the read", len(calls))
	}
}

func TestAll_BudgetStopsAnUnknownTotalIncomplete(t *testing.T) {
	t.Parallel()
	got, err := All(context.Background(), 2, 3, fetchFrom(corpus(10), nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 6 || got.Complete {
		t.Fatalf("items %d complete %v, want 6 and incomplete", len(got.Items), got.Complete)
	}
}

func TestAll_BudgetStopsAKnownTotalIncompleteAndKeepsTheTotal(t *testing.T) {
	t.Parallel()
	fetch := func(ctx context.Context, p Paging) (Batch[int], error) {
		b, err := fetchFrom(corpus(10), nil)(ctx, p)
		b.Total = ptr.To(10)
		return b, err
	}
	got, err := All(context.Background(), 2, 2, fetch)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 4 || got.Complete || got.Total == nil || *got.Total != 10 {
		t.Fatalf("items %d complete %v total %v, want 4, incomplete, 10", len(got.Items), got.Complete, got.Total)
	}
}

// A total that the pages never reach must not spin: an empty page ends the
// read, and the read is incomplete rather than presented as whole.
func TestAll_EmptyPageBeforeTheTotalIsIncomplete(t *testing.T) {
	t.Parallel()
	fetch := func(_ context.Context, p Paging) (Batch[int], error) {
		if p.Page == 1 {
			return Batch[int]{Items: []int{1, 2}, Total: ptr.To(10)}, nil
		}
		return Batch[int]{Total: ptr.To(10)}, nil
	}
	got, err := All(context.Background(), 2, 10, fetch)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 2 || got.Complete {
		t.Fatalf("items %v complete %v, want 2 and incomplete", got.Items, got.Complete)
	}
}

func TestAll_FetchErrorSurfaces(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	fetch := func(_ context.Context, p Paging) (Batch[int], error) {
		if p.Page == 2 {
			return Batch[int]{}, boom
		}
		return Batch[int]{Items: []int{1, 2}}, nil
	}
	if _, err := All(context.Background(), 2, 10, fetch); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
}
