// SPDX-License-Identifier: GPL-3.0-or-later

package pull

import (
	"context"
	"strings"
	"testing"
)

func TestReviewLifecycle(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := float64(openPR(t, f, "reviewed"))

	created := call(t, h.CreatePullReviewFn, map[string]any{
		"index": idx, "state": "COMMENT", "body": "looks odd",
		"comments": `[{"path":"a.go","body":"why?","new_position":3}]`,
	})
	if !strings.Contains(created, "looks odd") {
		t.Fatalf("create result: %s", created)
	}
	if out := call(t, h.ListPullReviewsFn, map[string]any{"index": idx}); !strings.Contains(out, "looks odd") {
		t.Fatalf("list result: %s", out)
	}
	if out := call(t, h.GetPullReviewFn, map[string]any{"index": idx, "id": float64(1)}); !strings.Contains(out, "looks odd") {
		t.Fatalf("get result: %s", out)
	}
	if out := call(t, h.ListPullReviewCommentsFn, map[string]any{"index": idx, "id": float64(1)}); !strings.Contains(out, "why?") {
		t.Fatalf("comments result: %s", out)
	}
	if out := call(t, h.DismissPullReviewFn, map[string]any{"index": idx, "id": float64(1), "message": "stale"}); !strings.Contains(out, "dismissed") {
		t.Fatalf("dismiss result: %s", out)
	}
	if out := call(t, h.DeletePullReviewFn, map[string]any{"index": idx, "id": float64(1)}); !strings.Contains(out, "deleted") {
		t.Fatalf("delete result: %s", out)
	}
	if _, err := h.GetPullReviewFn(context.Background(), makeReq(map[string]any{"index": idx, "id": float64(1)})); err == nil {
		t.Fatal("a deleted review must be gone")
	}
}

func TestSubmitPendingReview(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := float64(openPR(t, f, "reviewed"))
	call(t, h.CreatePullReviewFn, map[string]any{"index": idx, "state": "PENDING", "body": "draft"})
	out := call(t, h.SubmitPullReviewFn, map[string]any{"index": idx, "id": float64(1), "state": "APPROVED"})
	if !strings.Contains(out, "APPROVED") {
		t.Fatalf("submit result: %s", out)
	}
}

func TestCreatePullReview_InvalidCommentsJSON(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := float64(openPR(t, f, "reviewed"))
	_, err := h.CreatePullReviewFn(context.Background(), makeReq(map[string]any{
		"index": idx, "state": "COMMENT", "body": "x", "comments": "not json",
	}))
	if err == nil || !strings.Contains(err.Error(), "invalid comments JSON") {
		t.Fatalf("got %v", err)
	}
}

func TestReviewRequests(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := float64(openPR(t, f, "reviewed"))
	call(t, h.CreateReviewRequestsFn, map[string]any{"index": idx, "reviewers": "bob, carol"})
	list := call(t, h.ListPullReviewsFn, map[string]any{"index": idx})
	if !strings.Contains(list, "bob") || !strings.Contains(list, "carol") {
		t.Fatalf("requested reviewers should be listed: %s", list)
	}
	call(t, h.DeleteReviewRequestsFn, map[string]any{"index": idx, "reviewers": "bob"})
	list = call(t, h.ListPullReviewsFn, map[string]any{"index": idx})
	if strings.Contains(list, "bob") || !strings.Contains(list, "carol") {
		t.Fatalf("bob's request should be gone, carol's kept: %s", list)
	}
}
