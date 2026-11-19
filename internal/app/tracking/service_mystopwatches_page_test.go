// SPDX-License-Identifier: GPL-3.0-or-later

package tracking_test

import (
	"context"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/model"
	apptracking "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/tracking"
)

func myStopwatches(n int) []*model.StopWatch {
	out := make([]*model.StopWatch, n)
	for i := range out {
		out[i] = &model.StopWatch{IssueIndex: int64(i + 1), Seconds: int64(60 * (i + 1))}
	}
	return out
}

func TestMyStopwatchesReturnsOnePageWithItsNextMarker(t *testing.T) {
	t.Parallel()
	port := &stubPort{watches: myStopwatches(5)}
	s := apptracking.New(func(context.Context) (apptracking.Port, error) { return port, nil })
	first, err := s.MyStopwatches(context.Background(), apptracking.MyStopwatchesInput{Page: 1, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Stopwatches) != 2 || first.Stopwatches[0].IssueIndex != 1 || !first.HasNext || first.TotalCount != nil {
		t.Fatalf("page 1: %+v", first)
	}
	last, err := s.MyStopwatches(context.Background(), apptracking.MyStopwatchesInput{Page: 3, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(last.Stopwatches) != 1 || last.Stopwatches[0].IssueIndex != 5 || last.HasNext {
		t.Fatalf("last page: %+v", last)
	}
}

func TestMyStopwatchesDefaultPageHoldsAShortList(t *testing.T) {
	t.Parallel()
	port := &stubPort{watches: myStopwatches(3)}
	s := apptracking.New(func(context.Context) (apptracking.Port, error) { return port, nil })
	got, err := s.MyStopwatches(context.Background(), apptracking.MyStopwatchesInput{})
	if err != nil || len(got.Stopwatches) != 3 || got.HasNext || got.Limit != 20 {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestMyStopwatchesEmptyListIsAnEmptyArrayNotNull(t *testing.T) {
	t.Parallel()
	port := &stubPort{}
	s := apptracking.New(func(context.Context) (apptracking.Port, error) { return port, nil })
	got, err := s.MyStopwatches(context.Background(), apptracking.MyStopwatchesInput{})
	if err != nil || got.Stopwatches == nil || len(got.Stopwatches) != 0 || got.HasNext {
		t.Fatalf("got %+v, %v", got, err)
	}
}
