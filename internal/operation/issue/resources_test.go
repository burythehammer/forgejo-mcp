// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/resource"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

type readFn func(context.Context, mcp.ReadResourceRequest) ([]mcp.ResourceContents, error)

func readURI(t *testing.T, fn readFn, uri string) []mcp.ResourceContents {
	t.Helper()
	contents, err := fn(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: uri}})
	if err != nil {
		t.Fatalf("read %s: %v", uri, err)
	}
	return contents
}

func readJSON(t *testing.T, fn readFn, uri string, into any) {
	t.Helper()
	contents := readURI(t, fn, uri)
	block := contents[0].(mcp.TextResourceContents)
	if block.MIMEType != "application/json" {
		t.Errorf("MIME = %q", block.MIMEType)
	}
	if err := json.Unmarshal([]byte(block.Text), into); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, block.Text)
	}
}

func issueURI(index int64) string {
	return fmt.Sprintf("forgejo://repo/%s/%s/issue/%d", testOwner, testRepo, index)
}

func TestIssueResource_HappyPath(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	due := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	ms := f.SeedMilestone(testOwner, testRepo, "v1")
	bug := f.SeedIssueLabel(testOwner, testRepo, "bug")
	iss, err := f.CreateIssue(context.Background(), testOwner, testRepo, forgejo_sdk.CreateIssueOption{
		Title: "the title", Body: "the body", Ref: "refs/heads/dev", Assignees: []string{"bob"}, Milestone: ms, Labels: []int64{bug}, Deadline: &due,
	})
	if err != nil {
		t.Fatal(err)
	}
	comment(t, f, iss.Index, strings.Repeat("x", 300))

	var p issueResourcePayload
	readJSON(t, h.issueResourceHandler, issueURI(iss.Index), &p)
	if p.Title != "the title" || p.State != "open" || p.Ref != "refs/heads/dev" || p.Milestone != "v1" || p.DueDate != "2030-01-02T03:04:05Z" {
		t.Fatalf("payload %+v", p)
	}
	if len(p.Labels) != 1 || p.Labels[0] != "bug" || len(p.Assignees) != 1 || p.Assignees[0] != "bob" {
		t.Fatalf("labels/assignees %+v", p)
	}
	if p.CommentCount != 1 || len(p.RecentComments) != 1 || len(p.RecentComments[0].BodyExcerpt) >= 300 || p.Truncated {
		t.Fatalf("comments %+v", p)
	}
	md := readURI(t, h.issueResourceHandler, issueURI(iss.Index))[1].(mcp.TextResourceContents)
	if md.MIMEType != "text/markdown" || !strings.Contains(md.Text, "the body") {
		t.Fatalf("sidecar %+v", md)
	}
}

func TestIssueResource_OmitsUnsetFields(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := openIssue(t, f, "plain")
	block := readURI(t, h.issueResourceHandler, issueURI(idx))[0].(mcp.TextResourceContents)
	if strings.Contains(block.Text, "due_date") || strings.Contains(block.Text, `"ref"`) {
		t.Fatalf("unset fields leaked: %s", block.Text)
	}
}

func TestIssueResource_OverCapIsTruncated(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := openIssue(t, f, "busy")
	for i := 0; i < resource.EmbeddedListCap+1; i++ {
		comment(t, f, idx, fmt.Sprint("c", i))
	}
	var p issueResourcePayload
	readJSON(t, h.issueResourceHandler, issueURI(idx), &p)
	if !p.Truncated || p.ListTool != "list_issue_comments" || len(p.RecentComments) != resource.EmbeddedListCap {
		t.Fatalf("truncated=%v tool=%q n=%d", p.Truncated, p.ListTool, len(p.RecentComments))
	}
}

func TestIssueResource_Errors(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	for _, uri := range []string{issueURI(99), "forgejo://repo/octo/hello/issue/abc"} {
		if _, err := h.issueResourceHandler(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: uri}}); err == nil {
			t.Errorf("%s: want an error", uri)
		}
	}
}

func TestCommentResource(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := openIssue(t, f, "t")
	id := comment(t, f, idx, "hello there")
	var p commentResourcePayload
	readJSON(t, h.commentResourceHandler, fmt.Sprintf("forgejo://repo/octo/hello/issue/%d/comment/%d", idx, id), &p)
	if p.Body != "hello there" || p.Kind != "issue" || p.ID != id {
		t.Fatalf("payload %+v", p)
	}
	for _, uri := range []string{
		fmt.Sprintf("forgejo://repo/octo/hello/widget/%d/comment/%d", idx, id),
		fmt.Sprintf("forgejo://repo/octo/hello/issue/%d/comment/999", idx),
	} {
		if _, err := h.commentResourceHandler(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: uri}}); err == nil {
			t.Errorf("%s: want an error", uri)
		}
	}
}

func listURI(query string) string {
	return fmt.Sprintf("forgejo://repo/%s/%s/issues%s", testOwner, testRepo, query)
}

func TestRepoIssuesResource_RowsAndFilters(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	a := openIssue(t, f, "alpha")
	b := openIssue(t, f, "beta")
	bug := f.SeedIssueLabel(testOwner, testRepo, "Kind/Bug")
	if err := f.ReplaceIssueLabels(context.Background(), testOwner, testRepo, b, []int64{bug}); err != nil {
		t.Fatal(err)
	}
	closed := forgejo_sdk.StateClosed
	if _, err := f.EditIssue(context.Background(), testOwner, testRepo, a, forgejo_sdk.EditIssueOption{State: &closed}); err != nil {
		t.Fatal(err)
	}

	var p issuesListPayload
	readJSON(t, h.repoIssuesResourceHandler, listURI(""), &p)
	if p.State != "open" || len(p.Issues) != 1 || p.Issues[0].Index != b || p.Truncated {
		t.Fatalf("default: %+v", p)
	}
	if block := readURI(t, h.repoIssuesResourceHandler, listURI(""))[0].(mcp.TextResourceContents); strings.Contains(block.Text, "body of") {
		t.Fatalf("rows must carry no bodies: %s", block.Text)
	}
	p = issuesListPayload{}
	readJSON(t, h.repoIssuesResourceHandler, listURI("?state=all"), &p)
	if len(p.Issues) != 2 {
		t.Fatalf("all: %+v", p)
	}
	p = issuesListPayload{}
	readJSON(t, h.repoIssuesResourceHandler, listURI("?state=all&labels=Kind%2FBug"), &p)
	if len(p.Issues) != 1 || p.Issues[0].Index != b || len(p.Issues[0].Labels) != 1 {
		t.Fatalf("labels: %+v", p)
	}
	p = issuesListPayload{}
	readJSON(t, h.repoIssuesResourceHandler, listURI("?state=bogus"), &p)
	if p.State != "open" {
		t.Fatalf("unknown state falls back to open: %q", p.State)
	}
}

func TestRepoIssuesResource_PagesCoverEveryRow(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	const total, limit = 7, 3
	for i := 0; i < total; i++ {
		openIssue(t, f, fmt.Sprint("i", i))
	}
	seen := map[int64]bool{}
	for page := 1; page <= 3; page++ {
		var p issuesListPayload
		readJSON(t, h.repoIssuesResourceHandler, listURI(fmt.Sprintf("?page=%d&limit=%d", page, limit)), &p)
		for _, r := range p.Issues {
			if seen[r.Index] {
				t.Fatalf("issue %d on two pages", r.Index)
			}
			seen[r.Index] = true
		}
		wantMore := page < 3
		if p.Truncated != wantMore {
			t.Fatalf("page %d truncated=%v, want %v (%+v)", page, p.Truncated, wantMore, p)
		}
		if wantMore && (p.ListTool != "list_repo_issues" || p.Sentinel == "") {
			t.Fatalf("page %d sentinel missing: %+v", page, p)
		}
	}
	if len(seen) != total {
		t.Fatalf("saw %d of %d rows", len(seen), total)
	}
}

func TestRepoIssuesResource_Errors(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	if _, err := h.repoIssuesResourceHandler(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "forgejo://repo/octo/missing/issues"}}); err == nil {
		t.Fatal("unknown repo should error")
	}
	if _, err := h.repoIssuesResourceHandler(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: "forgejo://bogus"}}); err == nil {
		t.Fatal("bad URI should error")
	}
}

func commentsURI(kind string, index int64, query string) string {
	return fmt.Sprintf("forgejo://repo/%s/%s/%s/%d/comments%s", testOwner, testRepo, kind, index, query)
}

func TestIssueCommentsResource_FullBodiesAndPaging(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := openIssue(t, f, "t")
	long := strings.Repeat("y", 500)
	comment(t, f, idx, long)
	for i := 1; i < 5; i++ {
		comment(t, f, idx, fmt.Sprint("c", i))
	}
	var p commentsListPayload
	readJSON(t, h.issueCommentsResourceHandler, commentsURI("issue", idx, "?limit=2"), &p)
	if len(p.Comments) != 2 || p.Comments[0].Body != long || !p.Truncated || p.ListTool != "list_issue_comments" {
		t.Fatalf("page 1: %+v", p)
	}
	p = commentsListPayload{}
	readJSON(t, h.issueCommentsResourceHandler, commentsURI("issue", idx, "?limit=2&page=3"), &p)
	if len(p.Comments) != 1 || p.Comments[0].Body != "c4" || p.Truncated {
		t.Fatalf("page 3: %+v", p)
	}
	p = commentsListPayload{}
	readJSON(t, h.issueCommentsResourceHandler, commentsURI("issue", idx, "?limit=2&page=9"), &p)
	if len(p.Comments) != 0 || p.Truncated {
		t.Fatalf("past the end: %+v", p)
	}
}

func TestIssueCommentsResource_Errors(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := openIssue(t, f, "t")
	for _, uri := range []string{commentsURI("widget", idx, ""), "forgejo://repo/octo/hello/issue/abc/comments"} {
		if _, err := h.issueCommentsResourceHandler(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: uri}}); err == nil {
			t.Errorf("%s: want an error", uri)
		}
	}
}

// boundsIgnoring is a port whose comment list does what Forgejo's real
// endpoint does: it hands back the whole thread whatever page and limit are
// asked for, and reports the thread's size in X-Total-Count.
type boundsIgnoring struct{ *fake.Forgejo }

func (b boundsIgnoring) ListIssueComments(ctx context.Context, owner, repo string, index int64, opt forgejo_sdk.ListIssueCommentOptions) ([]*forgejo_sdk.Comment, forge.PageInfo, error) {
	return b.Forgejo.ListIssueComments(ctx, owner, repo, index, forgejo_sdk.ListIssueCommentOptions{ListOptions: forgejo_sdk.ListOptions{Page: -1}})
}

func TestIssueCommentsResource_ServerIgnoringBoundsIsPagedByTheHandler(t *testing.T) {
	t.Parallel()
	f := fake.New()
	f.AddRepo(testOwner, testRepo)
	idx := openIssue(t, f, "t")
	for i := 0; i < 7; i++ {
		comment(t, f, idx, fmt.Sprint("c", i))
	}
	h := withPort(boundsIgnoring{f})

	var p commentsListPayload
	readJSON(t, h.issueCommentsResourceHandler, commentsURI("pr", idx, "?limit=3&page=2"), &p)
	if len(p.Comments) != 3 || p.Comments[0].Body != "c3" || !p.Truncated {
		t.Fatalf("page 2: %+v", p)
	}
	p = commentsListPayload{}
	readJSON(t, h.issueCommentsResourceHandler, commentsURI("pr", idx, "?limit=3&page=3"), &p)
	if len(p.Comments) != 1 || p.Comments[0].Body != "c6" || p.Truncated {
		t.Fatalf("page 3: %+v", p)
	}
	p = commentsListPayload{}
	readJSON(t, h.issueCommentsResourceHandler, commentsURI("pr", idx, "?limit=3&page=4"), &p)
	if len(p.Comments) != 0 {
		t.Fatalf("past the end: %+v", p)
	}
}
