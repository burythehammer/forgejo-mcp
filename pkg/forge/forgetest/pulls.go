// SPDX-License-Identifier: GPL-3.0-or-later

package forgetest

import (
	"context"
	"fmt"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// RunPulls runs the pull-request contract against the implementation that
// newHarness builds. newHarness is called once per case, so cases never share
// state and run in parallel.
func RunPulls(t *testing.T, newHarness func(t *testing.T) Harness) {
	cases := []struct {
		name string
		run  func(t *testing.T, h Harness)
	}{
		{"get unknown pull request is not found", func(t *testing.T, h Harness) {
			_, err := h.Port.GetPullRequest(ctx, Owner, Repo, 99)
			wantNotFound(t, err)
			_, err = h.Port.GetPullRequest(ctx, "nobody", "nothing", 1)
			wantNotFound(t, err)
		}},
		{"created pull request reads back", func(t *testing.T, h Harness) {
			a := newPR(t, h, "first", "feature-a")
			b := newPR(t, h, "second", "feature-b")
			if a.Index == b.Index {
				t.Fatalf("indices must differ, both %d", a.Index)
			}
			got, err := h.Port.GetPullRequest(ctx, Owner, Repo, a.Index)
			must(t, err)
			if got.Title != "first" || got.Body != "body of first" || got.State != forgejo_sdk.StateOpen {
				t.Fatalf("got title=%q body=%q state=%q", got.Title, got.Body, got.State)
			}
			if got.Head == nil || got.Head.Ref != "feature-a" || got.Base == nil || got.Base.Ref != "main" {
				t.Fatalf("got head=%+v base=%+v", got.Head, got.Base)
			}
			if got.HasMerged {
				t.Fatal("a new pull request is not merged")
			}
		}},
		{"create without a title is rejected", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			_, err := h.Port.CreatePullRequest(ctx, Owner, Repo, forgejo_sdk.CreatePullRequestOption{Head: "h", Base: "main"})
			if err == nil {
				t.Fatal("want an error")
			}
		}},
		{"create in an unknown repository is not found", func(t *testing.T, h Harness) {
			_, err := h.Port.CreatePullRequest(ctx, "nobody", "nothing", forgejo_sdk.CreatePullRequestOption{Head: "h", Base: "main", Title: "t"})
			wantNotFound(t, err)
		}},
		{"list defaults to open, newest first", func(t *testing.T, h Harness) {
			a := newPR(t, h, "a", "fa")
			b := newPR(t, h, "b", "fb")
			c := newPR(t, h, "c", "fc")
			mergeNow(t, h, b.Index)

			got, err := h.Port.ListPullRequests(ctx, Owner, Repo, forgejo_sdk.ListPullRequestsOptions{})
			must(t, err)
			wantIndices(t, got, c.Index, a.Index)
		}},
		{"list filters by state and sorts", func(t *testing.T, h Harness) {
			a := newPR(t, h, "a", "fa")
			b := newPR(t, h, "b", "fb")
			mergeNow(t, h, a.Index)

			closed, err := h.Port.ListPullRequests(ctx, Owner, Repo, forgejo_sdk.ListPullRequestsOptions{State: forgejo_sdk.StateClosed})
			must(t, err)
			wantIndices(t, closed, a.Index)

			all, err := h.Port.ListPullRequests(ctx, Owner, Repo, forgejo_sdk.ListPullRequestsOptions{State: forgejo_sdk.StateAll, Sort: "oldest"})
			must(t, err)
			wantIndices(t, all, a.Index, b.Index)
		}},
		{"list paginates", func(t *testing.T, h Harness) {
			var want []int64
			for i := 0; i < 5; i++ {
				want = append([]int64{newPR(t, h, fmt.Sprint("pr", i), fmt.Sprint("f", i)).Index}, want...) // newest first
			}
			page := func(p int) []*forgejo_sdk.PullRequest {
				got, err := h.Port.ListPullRequests(ctx, Owner, Repo, forgejo_sdk.ListPullRequestsOptions{
					ListOptions: forgejo_sdk.ListOptions{Page: p, PageSize: 2},
				})
				must(t, err)
				return got
			}
			wantIndices(t, page(1), want[0], want[1])
			wantIndices(t, page(2), want[2], want[3])
			wantIndices(t, page(3), want[4])
			wantIndices(t, page(4))
		}},
		{"list in an unknown repository is not found", func(t *testing.T, h Harness) {
			_, err := h.Port.ListPullRequests(ctx, "nobody", "nothing", forgejo_sdk.ListPullRequestsOptions{})
			wantNotFound(t, err)
		}},
		{"edit changes what was named and keeps the rest", func(t *testing.T, h Harness) {
			pr := newPR(t, h, "old title", "feat")
			body := "new body"
			got, err := h.Port.EditPullRequest(ctx, Owner, Repo, pr.Index, forgejo_sdk.EditPullRequestOption{Title: "new title", Body: &body})
			must(t, err)
			if got.Title != "new title" || got.Body != "new body" {
				t.Fatalf("edit answered title=%q body=%q", got.Title, got.Body)
			}
			read, err := h.Port.GetPullRequest(ctx, Owner, Repo, pr.Index)
			must(t, err)
			if read.Title != "new title" || read.Body != "new body" || read.Head.Ref != "feat" {
				t.Fatalf("read back title=%q body=%q head=%+v", read.Title, read.Body, read.Head)
			}
		}},
		{"edit unknown pull request is not found", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			_, err := h.Port.EditPullRequest(ctx, Owner, Repo, 99, forgejo_sdk.EditPullRequestOption{Title: "x"})
			wantNotFound(t, err)
		}},
		{"merge closes the pull request as merged, once", func(t *testing.T, h Harness) {
			pr := newPR(t, h, "m", "fm")
			res, err := h.Port.MergePullRequest(ctx, Owner, Repo, pr.Index, forgejo_sdk.MergePullRequestOption{Style: forgejo_sdk.MergeStyleMerge})
			must(t, err)
			if !res.Merged || res.StatusCode != 200 || res.Message != "" {
				t.Fatalf("got %+v, want merged with 200", res)
			}
			got, err := h.Port.GetPullRequest(ctx, Owner, Repo, pr.Index)
			must(t, err)
			if !got.HasMerged || got.State != forgejo_sdk.StateClosed {
				t.Fatalf("after merge: merged=%v state=%q", got.HasMerged, got.State)
			}
			again, err := h.Port.MergePullRequest(ctx, Owner, Repo, pr.Index, forgejo_sdk.MergePullRequestOption{Style: forgejo_sdk.MergeStyleMerge})
			must(t, err)
			if again.Merged || again.StatusCode != 405 {
				t.Fatalf("second merge got %+v, want declined with 405", again)
			}
		}},
		{"merge when checks succeed schedules without merging", func(t *testing.T, h Harness) {
			pr := newPR(t, h, "m", "fm")
			res, err := h.Port.MergePullRequest(ctx, Owner, Repo, pr.Index, forgejo_sdk.MergePullRequestOption{
				Style: forgejo_sdk.MergeStyleMerge, MergeWhenChecksSucceed: true,
			})
			must(t, err)
			if res.Merged || res.StatusCode != 201 || res.Message != "" {
				t.Fatalf("got %+v, want scheduled (201, not merged)", res)
			}
			got, err := h.Port.GetPullRequest(ctx, Owner, Repo, pr.Index)
			must(t, err)
			if got.HasMerged {
				t.Fatal("a scheduled merge has not merged yet")
			}
		}},
		{"merge unknown pull request reports 404", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			res, err := h.Port.MergePullRequest(ctx, Owner, Repo, 99, forgejo_sdk.MergePullRequestOption{Style: forgejo_sdk.MergeStyleMerge})
			must(t, err)
			if res.Merged || res.StatusCode != 404 {
				t.Fatalf("got %+v, want 404", res)
			}
		}},
		{"declined merge carries Forgejo's own message", func(t *testing.T, h Harness) {
			pr := newPR(t, h, "m", "fm")
			for _, c := range []struct {
				status  int
				message string
			}{
				{422, "Pull request has merge conflicts"},
				{405, "Pull request is not mergeable"},
				{409, "Head branch was updated"},
			} {
				must(t, h.Seed.SetMergeDecline(Owner, Repo, pr.Index, c.status, c.message))
				res, err := h.Port.MergePullRequest(ctx, Owner, Repo, pr.Index, forgejo_sdk.MergePullRequestOption{Style: forgejo_sdk.MergeStyleMerge})
				must(t, err)
				if res.Merged || res.StatusCode != c.status || res.Message != c.message {
					t.Fatalf("got %+v, want declined with %d %q", res, c.status, c.message)
				}
			}
		}},
		{"declined merge stays declined when checks succeed is set", func(t *testing.T, h Harness) {
			pr := newPR(t, h, "m", "fm")
			must(t, h.Seed.SetMergeDecline(Owner, Repo, pr.Index, 422, "Pull request has merge conflicts"))
			res, err := h.Port.MergePullRequest(ctx, Owner, Repo, pr.Index, forgejo_sdk.MergePullRequestOption{
				Style: forgejo_sdk.MergeStyleMerge, MergeWhenChecksSucceed: true,
			})
			must(t, err)
			if res.Merged || res.StatusCode != 422 || res.Message != "Pull request has merge conflicts" {
				t.Fatalf("got %+v, want declined with 422 and Forgejo's message", res)
			}
		}},
		{"declined merge whose body is not JSON reports the status alone", func(t *testing.T, h Harness) {
			pr := newPR(t, h, "m", "fm")
			must(t, h.Seed.SetMergeDecline(Owner, Repo, pr.Index, 422, ""))
			res, err := h.Port.MergePullRequest(ctx, Owner, Repo, pr.Index, forgejo_sdk.MergePullRequestOption{Style: forgejo_sdk.MergeStyleMerge})
			must(t, err)
			if res.Merged || res.StatusCode != 422 || res.Message != "" {
				t.Fatalf("got %+v, want declined with 422 and no message", res)
			}
		}},
		{"diff is returned verbatim", func(t *testing.T, h Harness) {
			pr := newPR(t, h, "d", "fd")
			const diff = "diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1 +1 @@\n-a\n+b\n"
			must(t, h.Seed.SetPullRequestDiff(Owner, Repo, pr.Index, diff))
			got, err := h.Port.GetPullRequestDiff(ctx, Owner, Repo, pr.Index)
			must(t, err)
			if string(got) != diff {
				t.Fatalf("got %q", got)
			}
			_, err = h.Port.GetPullRequestDiff(ctx, Owner, Repo, 99)
			wantNotFound(t, err)
		}},
		{"changed files paginate", func(t *testing.T, h Harness) {
			pr := newPR(t, h, "f", "ff")
			must(t, h.Seed.SetPullRequestFiles(Owner, Repo, pr.Index,
				&forgejo_sdk.ChangedFile{Filename: "a.go", Status: "added"},
				&forgejo_sdk.ChangedFile{Filename: "b.go", Status: "modified"},
				&forgejo_sdk.ChangedFile{Filename: "c.go", Status: "deleted"},
			))
			first, err := h.Port.ListPullRequestFiles(ctx, Owner, Repo, pr.Index, forgejo_sdk.ListPullRequestFilesOptions{ListOptions: forgejo_sdk.ListOptions{Page: 1, PageSize: 2}})
			must(t, err)
			second, err := h.Port.ListPullRequestFiles(ctx, Owner, Repo, pr.Index, forgejo_sdk.ListPullRequestFilesOptions{ListOptions: forgejo_sdk.ListOptions{Page: 2, PageSize: 2}})
			must(t, err)
			if len(first) != 2 || first[0].Filename != "a.go" || first[1].Filename != "b.go" || len(second) != 1 || second[0].Filename != "c.go" || second[0].Status != "deleted" {
				t.Fatalf("got %v then %v", names(first), names(second))
			}
		}},
		{"comments read back in order and paginate", func(t *testing.T, h Harness) {
			pr := newPR(t, h, "c", "fc")
			for _, body := range []string{"one", "two", "three"} {
				if _, err := h.Seed.AddPullRequestComment(Owner, Repo, pr.Index, "alice", body); err != nil {
					t.Fatal(err)
				}
			}
			all, err := h.Port.ListPullRequestComments(ctx, Owner, Repo, pr.Index, forgejo_sdk.ListOptions{})
			must(t, err)
			if len(all) != 3 || all[0].Body != "one" || all[2].Body != "three" || all[0].Poster == nil || all[0].Poster.UserName != "alice" {
				t.Fatalf("got %+v", all)
			}
			page2, err := h.Port.ListPullRequestComments(ctx, Owner, Repo, pr.Index, forgejo_sdk.ListOptions{Page: 2, PageSize: 2})
			must(t, err)
			if len(page2) != 1 || page2[0].Body != "three" {
				t.Fatalf("page 2 got %+v", page2)
			}
			got, err := h.Port.GetPullRequest(ctx, Owner, Repo, pr.Index)
			must(t, err)
			if got.Comments != 3 {
				t.Fatalf("comment count %d, want 3", got.Comments)
			}
		}},
		{"comment on unknown pull request is not found", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			_, err := h.Port.ListPullRequestComments(ctx, Owner, Repo, 99, forgejo_sdk.ListOptions{})
			wantNotFound(t, err)
		}},
		{"review with inline comments reads back", func(t *testing.T, h Harness) {
			pr := newPR(t, h, "r", "fr")
			created, err := h.Port.CreatePullReview(ctx, Owner, Repo, pr.Index, forgejo_sdk.CreatePullReviewOptions{
				State: forgejo_sdk.ReviewStateComment, Body: "looks odd",
				Comments: []forgejo_sdk.CreatePullReviewComment{{Path: "a.go", Body: "why?", NewLineNum: 3}},
			})
			must(t, err)
			got, err := h.Port.GetPullReview(ctx, Owner, Repo, pr.Index, created.ID)
			must(t, err)
			if got.Body != "looks odd" || got.State != forgejo_sdk.ReviewStateComment {
				t.Fatalf("got body=%q state=%q", got.Body, got.State)
			}
			list, err := h.Port.ListPullReviews(ctx, Owner, Repo, pr.Index, forgejo_sdk.ListPullReviewsOptions{})
			must(t, err)
			if len(list) != 1 || list[0].ID != created.ID {
				t.Fatalf("list got %+v", list)
			}
			comments, err := h.Port.ListPullReviewComments(ctx, Owner, Repo, pr.Index, created.ID)
			must(t, err)
			if len(comments) != 1 || comments[0].Body != "why?" || comments[0].Path != "a.go" {
				t.Fatalf("comments got %+v", comments)
			}
		}},
		{"empty review is rejected", func(t *testing.T, h Harness) {
			pr := newPR(t, h, "r", "fr")
			_, err := h.Port.CreatePullReview(ctx, Owner, Repo, pr.Index, forgejo_sdk.CreatePullReviewOptions{State: forgejo_sdk.ReviewStateComment})
			if err == nil {
				t.Fatal("want an error for a comment review with no body")
			}
		}},
		{"unknown review is not found", func(t *testing.T, h Harness) {
			pr := newPR(t, h, "r", "fr")
			_, err := h.Port.GetPullReview(ctx, Owner, Repo, pr.Index, 999)
			wantNotFound(t, err)
			_, err = h.Port.ListPullReviewComments(ctx, Owner, Repo, pr.Index, 999)
			wantNotFound(t, err)
		}},
		{"pending review is submitted once", func(t *testing.T, h Harness) {
			pr := newPR(t, h, "r", "fr")
			pending, err := h.Port.CreatePullReview(ctx, Owner, Repo, pr.Index, forgejo_sdk.CreatePullReviewOptions{
				State: forgejo_sdk.ReviewStatePending, Body: "draft",
			})
			must(t, err)
			done, err := h.Port.SubmitPullReview(ctx, Owner, Repo, pr.Index, pending.ID, forgejo_sdk.SubmitPullReviewOptions{
				State: forgejo_sdk.ReviewStateApproved, Body: "fine",
			})
			must(t, err)
			if done.State != forgejo_sdk.ReviewStateApproved {
				t.Fatalf("state %q", done.State)
			}
			_, err = h.Port.SubmitPullReview(ctx, Owner, Repo, pr.Index, pending.ID, forgejo_sdk.SubmitPullReviewOptions{
				State: forgejo_sdk.ReviewStateApproved, Body: "again",
			})
			if err == nil {
				t.Fatal("submitting a submitted review must fail")
			}
		}},
		{"dismissed review is marked, deleted review is gone", func(t *testing.T, h Harness) {
			pr := newPR(t, h, "r", "fr")
			rev, err := h.Port.CreatePullReview(ctx, Owner, Repo, pr.Index, forgejo_sdk.CreatePullReviewOptions{
				State: forgejo_sdk.ReviewStateRequestChanges, Body: "no",
			})
			must(t, err)
			must(t, h.Port.DismissPullReview(ctx, Owner, Repo, pr.Index, rev.ID, forgejo_sdk.DismissPullReviewOptions{Message: "stale"}))
			got, err := h.Port.GetPullReview(ctx, Owner, Repo, pr.Index, rev.ID)
			must(t, err)
			if !got.Dismissed {
				t.Fatal("review should be dismissed")
			}
			must(t, h.Port.DeletePullReview(ctx, Owner, Repo, pr.Index, rev.ID))
			_, err = h.Port.GetPullReview(ctx, Owner, Repo, pr.Index, rev.ID)
			wantNotFound(t, err)
			wantNotFound(t, h.Port.DeletePullReview(ctx, Owner, Repo, pr.Index, rev.ID))
		}},
		{"review requests appear as requested reviews and can be cancelled", func(t *testing.T, h Harness) {
			pr := newPR(t, h, "r", "fr")
			opt := forgejo_sdk.PullReviewRequestOptions{Reviewers: []string{"bob"}}
			must(t, h.Port.CreateReviewRequests(ctx, Owner, Repo, pr.Index, opt))
			list, err := h.Port.ListPullReviews(ctx, Owner, Repo, pr.Index, forgejo_sdk.ListPullReviewsOptions{})
			must(t, err)
			if len(list) != 1 || list[0].State != forgejo_sdk.ReviewStateRequestReview || list[0].Reviewer == nil || list[0].Reviewer.UserName != "bob" {
				t.Fatalf("after request: %+v", list)
			}
			must(t, h.Port.DeleteReviewRequests(ctx, Owner, Repo, pr.Index, opt))
			list, err = h.Port.ListPullReviews(ctx, Owner, Repo, pr.Index, forgejo_sdk.ListPullReviewsOptions{})
			must(t, err)
			if len(list) != 0 {
				t.Fatalf("after cancel: %+v", list)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.run(t, newHarness(t))
		})
	}
}

var ctx = context.Background()

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func wantNotFound(t *testing.T, err error) {
	t.Helper()
	if !forge.IsNotFound(err) {
		t.Fatalf("got %v (status %d), want not found", err, forge.StatusCode(err))
	}
}

// newPR creates a pull request in Owner/Repo, making the repository first.
func newPR(t *testing.T, h Harness, title, head string) *forgejo_sdk.PullRequest {
	t.Helper()
	h.Seed.AddRepo(Owner, Repo)
	pr, err := h.Port.CreatePullRequest(ctx, Owner, Repo, forgejo_sdk.CreatePullRequestOption{
		Head: head, Base: "main", Title: title, Body: "body of " + title,
	})
	must(t, err)
	return pr
}

func mergeNow(t *testing.T, h Harness, index int64) {
	t.Helper()
	res, err := h.Port.MergePullRequest(ctx, Owner, Repo, index, forgejo_sdk.MergePullRequestOption{Style: forgejo_sdk.MergeStyleMerge})
	must(t, err)
	if !res.Merged {
		t.Fatalf("merge of #%d declined: %+v", index, res)
	}
}

func wantIndices(t *testing.T, got []*forgejo_sdk.PullRequest, want ...int64) {
	t.Helper()
	have := make([]int64, len(got))
	for i, pr := range got {
		have[i] = pr.Index
	}
	if fmt.Sprint(have) != fmt.Sprint(append([]int64{}, want...)) {
		t.Fatalf("got indices %v, want %v", have, want)
	}
}

func names(files []*forgejo_sdk.ChangedFile) []string {
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = f.Filename
	}
	return out
}
