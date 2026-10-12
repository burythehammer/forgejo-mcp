// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/resource"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

// issueFacts is what a caller learns about one issue. The tool and the resource
// encode it differently, but these facts must agree.
type issueFacts struct {
	Index                                      int64
	Title, State, Body, Ref, Author, Milestone string
	Labels, Assignees                          []string
	CommentCount                               int
	CreatedAt, UpdatedAt, ClosedAt, DueDate    string
}

// commentFact is one comment as a caller sees it: its id, author and text.
type commentFact struct {
	ID           int64
	Author, Body string
}

// parityStamp is the timestamp layout both paths emit.
func parityStamp(t time.Time) string { return t.Format("2006-01-02T15:04:05Z07:00") }

func nilIfEmpty(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	return s
}

func toolFactsOf(iss forgejo_sdk.Issue) issueFacts {
	f := issueFacts{
		Index: iss.Index, Title: iss.Title, State: string(iss.State), Body: iss.Body, Ref: iss.Ref,
		CommentCount: iss.Comments, CreatedAt: parityStamp(iss.Created), UpdatedAt: parityStamp(iss.Updated),
	}
	if iss.Poster != nil {
		f.Author = iss.Poster.UserName
	}
	if iss.Milestone != nil {
		f.Milestone = iss.Milestone.Title
	}
	for _, l := range iss.Labels {
		f.Labels = append(f.Labels, l.Name)
	}
	for _, a := range iss.Assignees {
		f.Assignees = append(f.Assignees, a.UserName)
	}
	if iss.Closed != nil {
		f.ClosedAt = parityStamp(*iss.Closed)
	}
	if iss.Deadline != nil {
		f.DueDate = parityStamp(*iss.Deadline)
	}
	return f
}

func resourceFactsOf(t *testing.T, p issueResourcePayload, sidecar string) issueFacts {
	t.Helper()
	_, body, ok := strings.Cut(sidecar, "\n\n")
	if !ok {
		t.Fatalf("markdown sidecar has no body separator: %q", sidecar)
	}
	return issueFacts{
		Index: p.Index, Title: p.Title, State: p.State, Body: body, Ref: p.Ref,
		Author: p.Author, Milestone: p.Milestone, CommentCount: p.CommentCount,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt, ClosedAt: p.ClosedAt, DueDate: p.DueDate,
		Labels: nilIfEmpty(p.Labels), Assignees: nilIfEmpty(p.Assignees),
	}
}

func TestIssueReadParity_ToolAndResourceAgree(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	ctx := context.Background()
	due := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	ms := f.SeedMilestone(testOwner, testRepo, "v1")
	bug := f.SeedIssueLabel(testOwner, testRepo, "bug")
	iss, err := f.CreateIssue(ctx, testOwner, testRepo, forgejo_sdk.CreateIssueOption{
		Title: "the title", Body: "the body", Ref: "refs/heads/dev", Assignees: []string{"bob"},
		Milestone: ms, Labels: []int64{bug}, Deadline: &due,
	})
	if err != nil {
		t.Fatal(err)
	}
	comment(t, f, iss.Index, "first")
	comment(t, f, iss.Index, "second")

	// Facts: the tool's issue and the resource's issue must carry the same ones.
	var toolIssue forgejo_sdk.Issue
	if err := unmarshalResult(call(t, h.GetIssueByIndexFn, map[string]any{"index": float64(iss.Index)}), &toolIssue); err != nil {
		t.Fatal(err)
	}
	var p issueResourcePayload
	readJSON(t, h.issueResourceHandler, issueURI(iss.Index), &p)
	sidecar := readURI(t, h.issueResourceHandler, issueURI(iss.Index))[1].(mcp.TextResourceContents).Text
	if got, want := resourceFactsOf(t, p, sidecar), toolFactsOf(toolIssue); !reflect.DeepEqual(got, want) {
		t.Fatalf("facts differ\nresource: %+v\ntool:     %+v", got, want)
	}

	// Comments: the resource embeds the thread the list tool returns.
	var listed []forgejo_sdk.Comment
	if err := unmarshalResult(call(t, h.ListIssueCommentsFn, map[string]any{"index": float64(iss.Index)}), &listed); err != nil {
		t.Fatal(err)
	}
	var fromTool, fromResource []commentFact
	for _, c := range listed {
		fromTool = append(fromTool, commentFact{ID: c.ID, Author: c.Poster.UserName, Body: c.Body})
	}
	for _, c := range p.RecentComments {
		fromResource = append(fromResource, commentFact{ID: c.ID, Author: c.Author, Body: c.BodyExcerpt})
	}
	if !reflect.DeepEqual(fromResource, fromTool) {
		t.Fatalf("comments differ\nresource: %+v\ntool:     %+v", fromResource, fromTool)
	}
}

func TestIssueReadParity_ErrorsMapTheSameCode(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	ctx := context.Background()

	_, toolErr := h.GetIssueByIndexFn(ctx, repoReq(map[string]any{"index": float64(99)}))
	_, resErr := h.issueResourceHandler(ctx, mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: issueURI(99)}})

	var tr, rr *resource.ResourceError
	if !errors.As(toolErr, &tr) || !errors.As(resErr, &rr) {
		t.Fatalf("want *resource.ResourceError from both paths, got tool=%v resource=%v", toolErr, resErr)
	}
	if tr.Code != rr.Code || rr.Code != -32003 {
		t.Fatalf("codes: tool=%d resource=%d, want both -32003", tr.Code, rr.Code)
	}
}
