// SPDX-License-Identifier: GPL-3.0-or-later

package pull

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/mcptest"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/resource"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/fake"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

// thread serves one pull request with its discussion and reviews at fixed
// timestamps, so the resource output can be pinned byte for byte. Everything
// else is the fake.
type thread struct {
	*fake.Forgejo
	comments []*forgejo_sdk.Comment
	reviews  []*forgejo_sdk.PullReview
}

func (t thread) GetPullRequest(_ context.Context, _, _ string, index int64) (*forgejo_sdk.PullRequest, error) {
	pr := fixedPR("chatty")
	pr.Index = index
	pr.Comments = len(t.comments)
	return pr, nil
}

// ListPullRequestComments returns the whole thread whatever paging is asked,
// as Forgejo's issue-comment endpoint does.
func (t thread) ListPullRequestComments(context.Context, string, string, int64, forgejo_sdk.ListOptions) ([]*forgejo_sdk.Comment, error) {
	return t.comments, nil
}

func (t thread) ListPullReviews(context.Context, string, string, int64, forgejo_sdk.ListPullReviewsOptions) ([]*forgejo_sdk.PullReview, error) {
	return t.reviews, nil
}

// failingThread serves the pull request but fails the thread listing.
type failingThread struct{ thread }

func (failingThread) ListPullRequestComments(context.Context, string, string, int64, forgejo_sdk.ListOptions) ([]*forgejo_sdk.Comment, error) {
	return nil, errors.New("comments boom")
}

// chattyFixture is a pull request with 31 comments and 31 reviews: each list is
// over the resource's cap of 30, and the first comment is long enough to be
// excerpted.
func chattyFixture() thread {
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	octo := &forgejo_sdk.User{ID: 1, UserName: "octo"}
	var cs []*forgejo_sdk.Comment
	var rs []*forgejo_sdk.PullReview
	for i := 1; i <= 31; i++ {
		body := fmt.Sprintf("comment %d", i)
		if i == 1 {
			body = strings.Repeat("x", 300)
		}
		cs = append(cs, &forgejo_sdk.Comment{ID: int64(i), Poster: octo, Body: body, Created: at, Updated: at})
		rs = append(rs, &forgejo_sdk.PullReview{
			ID: int64(100 + i), Reviewer: octo, State: forgejo_sdk.ReviewStateComment,
			Body: fmt.Sprintf("review %d", i), Submitted: at,
		})
	}
	return thread{Forgejo: fake.New(), comments: cs, reviews: rs}
}

// TestPRResourceGolden_RecentThreadAndTruncation pins what the pull-request
// resource does on purpose: comment and review bodies are excerpted, both lists
// are capped at 30, and each truncation names the tool that lists the rest.
func TestPRResourceGolden_RecentThreadAndTruncation(t *testing.T) {
	t.Parallel()
	h := handlers{forge: fake.Factory{Forgejo: chattyFixture()}}
	contents, err := h.prResourceHandler(context.Background(), prResourceRequest(7))
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, c := range contents {
		b.WriteString(c.(mcp.TextResourceContents).Text)
		b.WriteString("\n---\n")
	}
	mcptest.Golden(t, "resource_pr_recent_thread", []byte(b.String()))
}

// TestPRResource_ThreadFailureIsAnError: a failed thread listing used to be
// dropped, so the resource said "no comments" as if the thread were empty. It
// now reports the failure, as the read use case does.
func TestPRResource_ThreadFailureIsAnError(t *testing.T) {
	t.Parallel()
	h := handlers{forge: fake.Factory{Forgejo: failingThread{chattyFixture()}}}
	_, err := h.prResourceHandler(context.Background(), prResourceRequest(7))
	var re *resource.ResourceError
	if !errors.As(err, &re) {
		t.Fatalf("want the thread failure as a resource error, got %v", err)
	}
}
