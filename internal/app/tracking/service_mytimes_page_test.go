// SPDX-License-Identifier: GPL-3.0-or-later

package tracking_test

import (
	"context"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/model"
	apptracking "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/tracking"
)

func myTimes(n int) []*model.TrackedTime {
	out := make([]*model.TrackedTime, n)
	for i := range out {
		out[i] = &model.TrackedTime{ID: int64(i + 1), Time: int64(60 * (i + 1))}
	}
	return out
}

func TestMyTimesReturnsOnePageWithItsNextMarker(t *testing.T) {
	t.Parallel()
	port := &stubPort{times: myTimes(5)}
	s := apptracking.New(func(context.Context) (apptracking.Port, error) { return port, nil })
	first, err := s.MyTimes(context.Background(), apptracking.MyTimesInput{Page: 1, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Times) != 2 || first.Times[0].ID != 1 || !first.HasNext || first.TotalCount != nil {
		t.Fatalf("page 1: %+v", first)
	}
	last, err := s.MyTimes(context.Background(), apptracking.MyTimesInput{Page: 3, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(last.Times) != 1 || last.Times[0].ID != 5 || last.HasNext {
		t.Fatalf("last page: %+v", last)
	}
}

func TestMyTimesDefaultPageHoldsAShortHistory(t *testing.T) {
	t.Parallel()
	port := &stubPort{times: myTimes(3)}
	s := apptracking.New(func(context.Context) (apptracking.Port, error) { return port, nil })
	got, err := s.MyTimes(context.Background(), apptracking.MyTimesInput{})
	if err != nil || len(got.Times) != 3 || got.HasNext || got.Limit != 20 {
		t.Fatalf("got %+v, %v", got, err)
	}
}
