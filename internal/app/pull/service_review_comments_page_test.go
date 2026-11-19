// SPDX-License-Identifier: GPL-3.0-or-later

package pull_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	apppull "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/pull"
)

// pendingReviewWith creates a pending review carrying n inline comments, named
// c1..cn, and returns its ID.
func pendingReviewWith(t *testing.T, s *apppull.Service, i int64, n int) int64 {
	t.Helper()
	var parts []string
	for k := 1; k <= n; k++ {
		parts = append(parts, fmt.Sprintf(`{"path":"a.go","body":"c%d","new_position":%d}`, k, k))
	}
	rev, err := s.CreateReview(context.Background(), apppull.CreateReviewRequest{
		PR: pr(i), State: "PENDING", Body: "draft", Comments: "[" + strings.Join(parts, ",") + "]",
	})
	if err != nil {
		t.Fatal(err)
	}
	return rev.ID
}

func TestReviewCommentsPageWithNoGapOrOverlap(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	ctx := context.Background()
	i := open(t, s, "paged-review-comments")
	id := pendingReviewWith(t, s, i, 5)
	ref := apppull.ReviewRef{PR: pr(i), ID: id}

	first, err := s.ReviewComments(ctx, ref, apppull.Paging{Page: 1, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 2 || first.Items[0].Body != "c1" || !first.HasNext || first.Total == nil || *first.Total != 5 {
		t.Fatalf("page 1: %+v", first)
	}
	last, err := s.ReviewComments(ctx, ref, apppull.Paging{Page: 3, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(last.Items) != 1 || last.Items[0].Body != "c5" || last.HasNext {
		t.Fatalf("page 3: %+v", last)
	}
}

func TestReviewCommentsDefaultPageHoldsASmallReview(t *testing.T) {
	t.Parallel()
	s, _ := newService(t)
	ctx := context.Background()
	i := open(t, s, "small-review-comments")
	id := pendingReviewWith(t, s, i, 3)
	got, err := s.ReviewComments(ctx, apppull.ReviewRef{PR: pr(i), ID: id}, apppull.Paging{})
	if err != nil || got.Limit != 20 || len(got.Items) != 3 || got.HasNext {
		t.Fatalf("got %+v, %v", got, err)
	}
}
