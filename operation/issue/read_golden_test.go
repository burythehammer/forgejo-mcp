// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/mcptest"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/fake"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

// thread serves one issue and its comments with fixed timestamps, so the
// resource output can be pinned byte for byte. Everything else is the fake.
type thread struct {
	*fake.Forgejo
	issue    *forgejo_sdk.Issue
	comments []*forgejo_sdk.Comment
}

func (t thread) GetIssue(_ context.Context, _, _ string, index int64) (*forgejo_sdk.Issue, error) {
	if index != t.issue.Index {
		return nil, errors.New("issue not found")
	}
	return t.issue, nil
}

// ListIssueComments returns the whole thread whatever paging is asked, as
// Forgejo's issue-comment endpoint does.
func (t thread) ListIssueComments(context.Context, string, string, int64, forgejo_sdk.ListIssueCommentOptions) ([]*forgejo_sdk.Comment, forge.PageInfo, error) {
	return t.comments, forge.PageInfo{}, nil
}

// failingComments serves the issue but fails the comment listing.
type failingComments struct{ thread }

func (failingComments) ListIssueComments(context.Context, string, string, int64, forgejo_sdk.ListIssueCommentOptions) ([]*forgejo_sdk.Comment, forge.PageInfo, error) {
	return nil, forge.PageInfo{}, errors.New("comments boom")
}

// threadFixture is an issue with 31 comments: over the resource's cap of 30,
// and with a first comment long enough to be excerpted.
func threadFixture() thread {
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	octo := &forgejo_sdk.User{ID: 1, UserName: "octo"}
	iss := &forgejo_sdk.Issue{
		Index: 7, Title: "fixed", Body: "the body", State: forgejo_sdk.StateOpen,
		Poster: octo, Created: at, Updated: at, Comments: 31, Ref: "refs/heads/feat",
	}
	var cs []*forgejo_sdk.Comment
	for i := 1; i <= 31; i++ {
		body := fmt.Sprintf("comment %d", i)
		if i == 1 {
			body = strings.Repeat("x", 300)
		}
		cs = append(cs, &forgejo_sdk.Comment{ID: int64(i), Poster: octo, Body: body, Created: at, Updated: at})
	}
	return thread{Forgejo: fake.New(), issue: iss, comments: cs}
}

// TestIssueResourceGolden_RecentCommentsAndTruncation pins what the issue
// resource does on purpose and the tool does not: the body sits only in the
// markdown block, comment bodies are excerpted, the thread is capped at 30 and
// the truncation names list_issue_comments.
func TestIssueResourceGolden_RecentCommentsAndTruncation(t *testing.T) {
	t.Parallel()
	h := withPort(threadFixture())
	var b strings.Builder
	for _, c := range readURI(t, h.issueResourceHandler, issueURI(7)) {
		b.WriteString(c.(mcp.TextResourceContents).Text)
		b.WriteString("\n---\n")
	}
	mcptest.Golden(t, "resource_issue_recent_comments", []byte(b.String()))
}

// TestIssueResource_CommentFailureIsAnError: a failed comment listing used to
// be dropped, so the resource said "no comments" as if the thread were empty.
// It now reports the failure, as the read use case does.
func TestIssueResource_CommentFailureIsAnError(t *testing.T) {
	t.Parallel()
	h := withPort(failingComments{threadFixture()})
	_, err := h.issueResourceHandler(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: issueURI(7)}})
	if err == nil {
		t.Fatal("want the comment failure, got nil")
	}
}
