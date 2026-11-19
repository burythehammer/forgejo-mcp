// SPDX-License-Identifier: GPL-3.0-or-later

package list

import (
	"context"
	"errors"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/ptr"
)

func TestFileCeilingIs1000Rows(t *testing.T) {
	t.Parallel()
	if FileCeiling != 1000 {
		t.Fatalf("FileCeiling = %d, want 1000", FileCeiling)
	}
}

func TestReadWholeAtTheCeilingIsWhole(t *testing.T) {
	t.Parallel()
	got, err := ReadWhole(context.Background(), 3, 10, fetchFrom(corpus(10), nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 10 || got[0] != 1 || got[9] != 10 {
		t.Fatalf("rows = %v, want 1..10", got)
	}
}

func TestReadWholeOneOverFailsNamingTheCeiling(t *testing.T) {
	t.Parallel()
	got, err := ReadWhole(context.Background(), 3, 10, fetchFrom(corpus(11), nil))
	var over *OverCeiling
	if !errors.As(err, &over) || over.Max != 10 {
		t.Fatalf("err = %v, want *OverCeiling{Max: 10}", err)
	}
	if got != nil {
		t.Fatalf("over the ceiling returned rows %v, want none (never a truncated list)", got)
	}
}

func TestReadWholeReportedTotalOverCeilingFailsBeforeReadingRows(t *testing.T) {
	t.Parallel()
	var calls int
	fetch := func(ctx context.Context, p Paging) (Batch[int], error) {
		calls++
		b, err := fetchFrom(corpus(11), nil)(ctx, p)
		b.Total = ptr.To(11)
		return b, err
	}
	_, err := ReadWhole(context.Background(), 3, 10, fetch)
	var over *OverCeiling
	if !errors.As(err, &over) {
		t.Fatalf("err = %v, want *OverCeiling", err)
	}
	if calls != 1 {
		t.Fatalf("fetch calls = %d, want 1: a total over the ceiling is refused on the first page", calls)
	}
}

func TestReadWholeEmptyPageBeforeTheTotalIsAnError(t *testing.T) {
	t.Parallel()
	fetch := func(ctx context.Context, p Paging) (Batch[int], error) {
		b, err := fetchFrom(corpus(2), nil)(ctx, p)
		b.Total = ptr.To(5)
		return b, err
	}
	_, err := ReadWhole(context.Background(), 2, 10, fetch)
	if !errors.Is(err, ErrShortRead) {
		t.Fatalf("err = %v, want ErrShortRead", err)
	}
}

func TestReadWholePropagatesAFetchError(t *testing.T) {
	t.Parallel()
	boom := errors.New("boom")
	fetch := func(context.Context, Paging) (Batch[int], error) { return Batch[int]{}, boom }
	if _, err := ReadWhole(context.Background(), 3, 10, fetch); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
}
