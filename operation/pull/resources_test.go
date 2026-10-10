// SPDX-License-Identifier: GPL-3.0-or-later

package pull

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/resource"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/fake"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

func prResourceRequest(index int64) mcp.ReadResourceRequest {
	return mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{URI: fmt.Sprintf("forgejo://repo/%s/%s/pr/%d", testOwner, testRepo, index)},
	}
}

// readPR reads the resource and returns its JSON payload and markdown sidecar.
func readPR(t *testing.T, h handlers, index int64) (prResourcePayload, mcp.TextResourceContents) {
	t.Helper()
	contents, err := h.prResourceHandler(context.Background(), prResourceRequest(index))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(contents) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(contents))
	}
	block := contents[0].(mcp.TextResourceContents)
	if block.MIMEType != "application/json" {
		t.Errorf("MIME: got %q, want application/json", block.MIMEType)
	}
	var payload prResourcePayload
	if err := json.Unmarshal([]byte(block.Text), &payload); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	return payload, contents[1].(mcp.TextResourceContents)
}

func TestPRResourceHandler_OpenPR_HappyPath(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := openPR(t, f, "feat: add resources")
	for _, body := range []string{"first", "second"} {
		if _, err := f.AddPullRequestComment(testOwner, testRepo, idx, "bob", body); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.CreatePullReview(context.Background(), testOwner, testRepo, idx, forgejo_sdk.CreatePullReviewOptions{
		State: forgejo_sdk.ReviewStateApproved,
	}); err != nil {
		t.Fatal(err)
	}

	payload, md := readPR(t, h, idx)
	if payload.State != "open" || !payload.Mergeable {
		t.Errorf("state=%q mergeable=%v, want open and mergeable", payload.State, payload.Mergeable)
	}
	if payload.Title != "feat: add resources" || payload.Author != fake.Poster {
		t.Errorf("title=%q author=%q", payload.Title, payload.Author)
	}
	if payload.Head.Ref != "feature" || payload.Base.Ref != "main" {
		t.Errorf("head=%+v base=%+v", payload.Head, payload.Base)
	}
	if len(payload.RecentComments) != 2 || payload.RecentComments[0].Author != "bob" || payload.CommentCount != 2 {
		t.Errorf("comments=%+v count=%d", payload.RecentComments, payload.CommentCount)
	}
	if payload.ReviewCount != 1 || len(payload.RecentReviews) != 1 {
		t.Errorf("reviews=%+v count=%d", payload.RecentReviews, payload.ReviewCount)
	}
	if payload.CommentsTruncated || payload.ReviewsTruncated {
		t.Error("nothing should be truncated")
	}
	if md.MIMEType != "text/markdown" {
		t.Errorf("sidecar MIME: got %q, want text/markdown", md.MIMEType)
	}
	if !strings.Contains(md.Text, "feat: add resources") {
		t.Errorf("sidecar missing title, got %q", md.Text)
	}
}

func TestPRResourceHandler_MergedPR(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := openPR(t, f, "merged")
	if _, err := f.MergePullRequest(context.Background(), testOwner, testRepo, idx, forgejo_sdk.MergePullRequestOption{}); err != nil {
		t.Fatal(err)
	}
	payload, _ := readPR(t, h, idx)
	if payload.State != "merged" {
		t.Errorf("expected state=merged, got %q", payload.State)
	}
	if payload.MergedAt == "" {
		t.Error("expected merged_at to be set")
	}
}

func TestPRResourceHandler_OverCapComments(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := openPR(t, f, "chatty")
	for i := 0; i <= resource.EmbeddedListCap; i++ {
		if _, err := f.AddPullRequestComment(testOwner, testRepo, idx, "bob", "comment"); err != nil {
			t.Fatal(err)
		}
	}
	payload, _ := readPR(t, h, idx)
	if !payload.CommentsTruncated {
		t.Error("expected comments_truncated=true")
	}
	if payload.CommentsListTool != "list_issue_comments" {
		t.Errorf("expected comments_list_tool=list_issue_comments, got %q", payload.CommentsListTool)
	}
	if len(payload.RecentComments) != resource.EmbeddedListCap {
		t.Errorf("expected %d capped comments, got %d", resource.EmbeddedListCap, len(payload.RecentComments))
	}
}

func TestPRResourceHandler_OverCapReviews(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := openPR(t, f, "much reviewed")
	for i := 0; i <= resource.EmbeddedListCap; i++ {
		if _, err := f.CreatePullReview(context.Background(), testOwner, testRepo, idx, forgejo_sdk.CreatePullReviewOptions{
			State: forgejo_sdk.ReviewStateComment, Body: "review",
		}); err != nil {
			t.Fatal(err)
		}
	}
	payload, _ := readPR(t, h, idx)
	if !payload.ReviewsTruncated {
		t.Error("expected reviews_truncated=true")
	}
	if payload.ReviewsListTool != "list_pull_reviews" {
		t.Errorf("expected reviews_list_tool=list_pull_reviews, got %q", payload.ReviewsListTool)
	}
	if len(payload.RecentReviews) != resource.EmbeddedListCap {
		t.Errorf("expected %d capped reviews, got %d", resource.EmbeddedListCap, len(payload.RecentReviews))
	}
}

func TestPRResourceHandler_NonNumericIndex(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	_, err := h.prResourceHandler(context.Background(), mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{URI: "forgejo://repo/testowner/testrepo/pr/abc"},
	})
	var re *resource.ResourceError
	if !errors.As(err, &re) {
		t.Fatalf("expected *resource.ResourceError, got %T (%v)", err, err)
	}
	if re.Code != -32602 {
		t.Errorf("expected -32602, got %d", re.Code)
	}
}

func TestPRResourceHandler_NotFound(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	_, err := h.prResourceHandler(context.Background(), prResourceRequest(999))
	var re *resource.ResourceError
	if !errors.As(err, &re) {
		t.Fatalf("expected *resource.ResourceError, got %T (%v)", err, err)
	}
	if re.Code != -32003 {
		t.Errorf("expected -32003, got %d", re.Code)
	}
}
