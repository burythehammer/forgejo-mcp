// SPDX-License-Identifier: GPL-3.0-or-later

package pull

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/model"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/resource"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

// prFacts is what a caller learns about one pull request from the facts the
// compact read carries. The tool and the resource encode it differently, but
// these facts must agree. The poster and the creation time are verbose-only on
// the tool, so they are not compared here.
type prFacts struct {
	Index                         int64
	Title, State, Body            string
	HeadRef, BaseRef              string
	Mergeable                     bool
	CommentCount                  int
	UpdatedAt, ClosedAt, MergedAt string
}

// compactPR is get_pull_request_by_index's default result as a caller decodes it.
type compactPR struct {
	Number    int64      `json:"number"`
	Title     string     `json:"title"`
	State     string     `json:"state"`
	Head      string     `json:"head"`
	Base      string     `json:"base"`
	Updated   time.Time  `json:"updated_at"`
	Body      string     `json:"body"`
	MergedAt  *time.Time `json:"merged_at"`
	Closed    *time.Time `json:"closed_at"`
	Mergeable bool       `json:"mergeable"`
	Comments  int        `json:"comments"`
}

// commentFact is one comment of the thread as a caller sees it.
type commentFact struct {
	ID           int64
	Author, Body string
}

// reviewFact is one review as a caller sees it.
type reviewFact struct {
	ID                    int64
	Reviewer, State, Body string
}

func prStamp(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(resourceTimeFormat)
}

func toolPRFacts(pr compactPR) prFacts {
	return prFacts{
		Index: pr.Number, Title: pr.Title, State: pr.State, Body: pr.Body,
		HeadRef: pr.Head, BaseRef: pr.Base, Mergeable: pr.Mergeable, CommentCount: pr.Comments,
		UpdatedAt: prStamp(&pr.Updated), ClosedAt: prStamp(pr.Closed), MergedAt: prStamp(pr.MergedAt),
	}
}

func resourcePRFacts(t *testing.T, p prResourcePayload, sidecar string) prFacts {
	t.Helper()
	_, body, ok := strings.Cut(sidecar, "\n\n")
	if !ok {
		t.Fatalf("markdown sidecar has no body separator: %q", sidecar)
	}
	return prFacts{
		Index: p.Index, Title: p.Title, State: p.State, Body: body,
		HeadRef: p.Head.Ref, BaseRef: p.Base.Ref, Mergeable: p.Mergeable, CommentCount: p.CommentCount,
		UpdatedAt: p.UpdatedAt, ClosedAt: p.ClosedAt, MergedAt: p.MergedAt,
	}
}

// callJSON runs a tool handler and decodes the payload of its text result, which
// internal/to wraps in a Result envelope, into v.
func callJSON(t *testing.T, fn toolFn, args map[string]any, v any) {
	t.Helper()
	var wrapped struct{ Result json.RawMessage }
	if err := json.Unmarshal([]byte(call(t, fn, args)), &wrapped); err != nil {
		t.Fatalf("decoding tool result: %v", err)
	}
	if err := json.Unmarshal(wrapped.Result, v); err != nil {
		t.Fatalf("decoding tool payload: %v", err)
	}
}

func TestPRReadParity_ToolAndResourceAgree(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	ctx := context.Background()
	idx := openPR(t, f, "feat: parity")
	first, err := f.AddPullRequestComment(testOwner, testRepo, idx, "bob", "first")
	if err != nil {
		t.Fatal(err)
	}
	second, err := f.AddPullRequestComment(testOwner, testRepo, idx, "carol", "second")
	if err != nil {
		t.Fatal(err)
	}
	review, err := f.CreatePullReview(ctx, testOwner, testRepo, idx, forgejo_sdk.CreatePullReviewOptions{
		State: forgejo_sdk.ReviewStateApproved, Body: "lgtm",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Facts: the tool's compact read and the resource's pull request must carry the same ones.
	var toolPR compactPR
	callJSON(t, h.GetPullRequestByIndexFn, map[string]any{"index": float64(idx)}, &toolPR)
	var p prResourcePayload
	var sidecar string
	{
		contents, err := h.prResourceHandler(ctx, prResourceRequest(idx))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal([]byte(contents[0].(mcp.TextResourceContents).Text), &p); err != nil {
			t.Fatal(err)
		}
		sidecar = contents[1].(mcp.TextResourceContents).Text
	}
	if got, want := resourcePRFacts(t, p, sidecar), toolPRFacts(toolPR); !reflect.DeepEqual(got, want) {
		t.Fatalf("facts differ\nresource: %+v\ntool:     %+v", got, want)
	}

	// Comments: the resource embeds the seeded thread, in Forgejo's order.
	var fromResource []commentFact
	for _, c := range p.RecentComments {
		fromResource = append(fromResource, commentFact{ID: c.ID, Author: c.Author, Body: c.BodyExcerpt})
	}
	wantThread := []commentFact{
		{ID: first.ID, Author: "bob", Body: "first"},
		{ID: second.ID, Author: "carol", Body: "second"},
	}
	if !reflect.DeepEqual(fromResource, wantThread) {
		t.Fatalf("thread differs\nresource: %+v\nseeded:   %+v", fromResource, wantThread)
	}

	// Reviews: the resource embeds the list the list_pull_reviews tool returns.
	var listed model.PullReviewPage
	callJSON(t, h.ListPullReviewsFn, map[string]any{"index": float64(idx)}, &listed)
	var fromTool, fromRes []reviewFact
	for _, r := range listed.Reviews {
		fromTool = append(fromTool, reviewFact{ID: r.ID, Reviewer: r.Reviewer.UserName, State: r.State, Body: r.Body})
	}
	for _, r := range p.RecentReviews {
		fromRes = append(fromRes, reviewFact{ID: r.ID, Reviewer: r.Reviewer, State: r.State, Body: r.BodyExcerpt})
	}
	if !reflect.DeepEqual(fromRes, fromTool) {
		t.Fatalf("reviews differ\nresource: %+v\ntool:     %+v", fromRes, fromTool)
	}
	if len(fromTool) != 1 || fromTool[0].ID != review.ID {
		t.Fatalf("seeded review missing from list tool: %+v", fromTool)
	}
}

func TestPRReadParity_ErrorsMapTheSameCode(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	ctx := context.Background()

	_, toolErr := h.GetPullRequestByIndexFn(ctx, makeReq(map[string]any{"index": float64(99)}))
	_, resErr := h.prResourceHandler(ctx, prResourceRequest(99))

	var tr, rr *resource.ResourceError
	if !errors.As(toolErr, &tr) || !errors.As(resErr, &rr) {
		t.Fatalf("want *resource.ResourceError from both paths, got tool=%v resource=%v", toolErr, resErr)
	}
	if tr.Code != rr.Code || rr.Code != -32003 {
		t.Fatalf("codes: tool=%d resource=%d, want both -32003", tr.Code, rr.Code)
	}
}
