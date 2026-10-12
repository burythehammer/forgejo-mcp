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

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/resource"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

// prFacts is what a caller learns about one pull request. The tool and the
// resource encode it differently, but these facts must agree.
type prFacts struct {
	Index                                    int64
	Title, State, Body, Author               string
	HeadRef, BaseRef                         string
	Mergeable                                bool
	CommentCount                             int
	CreatedAt, UpdatedAt, ClosedAt, MergedAt string
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

func toolPRFacts(pr model.PullRequest) prFacts {
	f := prFacts{
		Index: pr.Index, Title: pr.Title, State: pr.State, Body: pr.Body,
		Mergeable: pr.Mergeable, CommentCount: pr.Comments,
		CreatedAt: prStamp(pr.Created), UpdatedAt: prStamp(pr.Updated),
		ClosedAt: prStamp(pr.Closed), MergedAt: prStamp(pr.Merged),
	}
	if pr.Poster != nil {
		f.Author = pr.Poster.UserName
	}
	if pr.Head != nil {
		f.HeadRef = pr.Head.Ref
	}
	if pr.Base != nil {
		f.BaseRef = pr.Base.Ref
	}
	return f
}

func resourcePRFacts(t *testing.T, p prResourcePayload, sidecar string) prFacts {
	t.Helper()
	_, body, ok := strings.Cut(sidecar, "\n\n")
	if !ok {
		t.Fatalf("markdown sidecar has no body separator: %q", sidecar)
	}
	return prFacts{
		Index: p.Index, Title: p.Title, State: p.State, Body: body, Author: p.Author,
		HeadRef: p.Head.Ref, BaseRef: p.Base.Ref, Mergeable: p.Mergeable, CommentCount: p.CommentCount,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt, ClosedAt: p.ClosedAt, MergedAt: p.MergedAt,
	}
}

// callJSON runs a tool handler and decodes the payload of its text result, which
// pkg/to wraps in a Result envelope, into v.
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

	// Facts: the tool's pull request and the resource's must carry the same ones.
	var toolPR model.PullRequest
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
