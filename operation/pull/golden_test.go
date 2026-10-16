// SPDX-License-Identifier: GPL-3.0-or-later

package pull

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/mcptest"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/fake"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// These tests pin the wire contract of every pull tool: its tool definition
// (schema snapshot) and its output and error text (golden), together with the
// call the tool made on the Forgejo port. They run against a stub port that
// returns fixed, fully populated data, so the bytes are deterministic.

var allTools = []mcp.Tool{
	GetPullRequestByIndexTool, ListRepoPullRequestsTool, CreatePullRequestTool, UpdatePullRequestTool,
	ListPullReviewsTool, GetPullReviewTool, ListPullReviewCommentsTool, MergePullRequestTool,
	ListPullRequestFilesTool, GetPullRequestDiffTool,
	CreatePullReviewTool, SubmitPullReviewTool, DismissPullReviewTool, DeletePullReviewTool,
	CreateReviewRequestsTool, DeleteReviewRequestsTool,
}

func TestSchemaSnapshots(t *testing.T) {
	t.Parallel()
	for _, tool := range allTools {
		t.Run(tool.Name, func(t *testing.T) { mcptest.SchemaSnapshot(t, tool) })
	}
}

var (
	t0   = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	t1   = time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	sha  = "0123456789abcdef0123456789abcdef01234567"
	user = &forgejo_sdk.User{ID: 1, UserName: "octo", FullName: "Octo Cat", Email: "octo@example.com", Created: t0, LastLogin: t1, IsActive: true}
)

func fixedPR(title string) *forgejo_sdk.PullRequest {
	repo := &forgejo_sdk.Repository{ID: 9, Name: "hello", FullName: "octo/hello", Owner: user, Created: t0, Updated: t1}
	return &forgejo_sdk.PullRequest{
		ID: 11, URL: "https://f.example/api/v1/repos/octo/hello/pulls/7", Index: 7, Poster: user,
		Title: title, Body: "body", Labels: []*forgejo_sdk.Label{{ID: 3, Name: "bug", Color: "ff0000"}},
		Milestone: &forgejo_sdk.Milestone{ID: 4, Title: "v1", Created: t0},
		Assignee:  user, Assignees: []*forgejo_sdk.User{user},
		State: forgejo_sdk.StateOpen, Comments: 2, HTMLURL: "https://f.example/octo/hello/pulls/7",
		Mergeable: true, Merged: &t1, MergedCommitID: &sha, MergedBy: user,
		Base:      &forgejo_sdk.PRBranchInfo{Name: "main", Ref: "main", Sha: sha, RepoID: 9, Repository: repo},
		Head:      &forgejo_sdk.PRBranchInfo{Name: "feat", Ref: "feat", Sha: sha, RepoID: 9, Repository: repo},
		MergeBase: sha, Deadline: &t1, Created: &t0, Updated: &t1, Closed: &t1,
	}
}

func fixedReview() *forgejo_sdk.PullReview {
	return &forgejo_sdk.PullReview{
		ID: 21, Reviewer: user, ReviewerTeam: &forgejo_sdk.Team{ID: 5, Name: "core", Organization: &forgejo_sdk.Organization{ID: 6, UserName: "octo-org"}},
		State: forgejo_sdk.ReviewStateApproved, Body: "lgtm", CommitID: sha, Official: true,
		CodeCommentsCount: 1, Submitted: t1, HTMLURL: "https://f.example/r/21", HTMLPullURL: "https://f.example/p/7",
	}
}

func fixedReviewComment() *forgejo_sdk.PullReviewComment {
	return &forgejo_sdk.PullReviewComment{
		ID: 31, Body: "nit", Reviewer: user, ReviewID: 21, Resolver: user, Created: t0, Updated: t1,
		Path: "a.go", CommitID: sha, OrigCommitID: sha, DiffHunk: "@@ -1 +1 @@", LineNum: 3, OldLineNum: 2,
		HTMLURL: "https://f.example/c/31", HTMLPullURL: "https://f.example/p/7",
	}
}

const stubDiff = `diff --git a/cmd/main.go b/cmd/main.go
--- a/cmd/main.go
+++ b/cmd/main.go
@@ -1 +1,2 @@
 package main
+import "fmt"
diff --git a/README.md b/README.md
--- a/README.md
+++ b/README.md
@@ -1 +1,2 @@
 hi
+there
`

// stubPulls is a forge.Forgejo whose pull domain returns fixed data and records
// the calls it receives. Any other domain would panic on the nil embed.
type stubPulls struct {
	forge.Forgejo
	mu    sync.Mutex
	calls []string
	fail  error
	merge forge.MergeResult
	// pr, when set, is the pull request GetPullRequest returns in place of
	// fixedPR.
	pr *forgejo_sdk.PullRequest
	// prs, when set, is the page ListPullRequests returns in place of the two
	// fixedPR entries.
	prs []*forgejo_sdk.PullRequest
}

// mergedPR is fixedPR as Forgejo reports a merged pull request: closed, with the
// merged flag set along with the merge time and commit.
func mergedPR() *forgejo_sdk.PullRequest {
	pr := fixedPR("fixed")
	pr.State = forgejo_sdk.StateClosed
	pr.HasMerged = true
	return pr
}

// unmergedPR is fixedPR as Forgejo reports a pull request that is still open: no
// merge, no merge commit, no close time.
func unmergedPR() *forgejo_sdk.PullRequest {
	pr := fixedPR("fixed")
	pr.State = forgejo_sdk.StateOpen
	pr.HasMerged, pr.Merged, pr.MergedCommitID, pr.MergedBy, pr.Closed = false, nil, nil, nil, nil
	return pr
}

func (s *stubPulls) rec(name string, args ...any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, _ := json.Marshal(args)
	s.calls = append(s.calls, name+" "+string(b))
	return s.fail
}

func (s *stubPulls) GetPullRequest(_ context.Context, o, r string, i int64) (*forgejo_sdk.PullRequest, error) {
	pr := s.pr
	if pr == nil {
		pr = fixedPR("fixed")
	}
	return pr, s.rec("GetPullRequest", o, r, i)
}

func (s *stubPulls) ListPullRequests(_ context.Context, o, r string, opt forgejo_sdk.ListPullRequestsOptions) ([]*forgejo_sdk.PullRequest, error) {
	prs := s.prs
	if prs == nil {
		prs = []*forgejo_sdk.PullRequest{fixedPR("one"), fixedPR("two")}
	}
	return prs, s.rec("ListPullRequests", o, r, opt)
}

func (s *stubPulls) CreatePullRequest(_ context.Context, o, r string, opt forgejo_sdk.CreatePullRequestOption) (*forgejo_sdk.PullRequest, error) {
	return fixedPR(opt.Title), s.rec("CreatePullRequest", o, r, opt)
}

func (s *stubPulls) EditPullRequest(_ context.Context, o, r string, i int64, opt forgejo_sdk.EditPullRequestOption) (*forgejo_sdk.PullRequest, error) {
	return fixedPR("edited"), s.rec("EditPullRequest", o, r, i, opt)
}

func (s *stubPulls) MergePullRequest(_ context.Context, o, r string, i int64, opt forgejo_sdk.MergePullRequestOption) (forge.MergeResult, error) {
	return s.merge, s.rec("MergePullRequest", o, r, i, opt)
}

func (s *stubPulls) GetPullRequestDiff(_ context.Context, o, r string, i int64) ([]byte, error) {
	return []byte(stubDiff), s.rec("GetPullRequestDiff", o, r, i)
}

func (s *stubPulls) ListPullRequestFiles(_ context.Context, o, r string, i int64, opt forgejo_sdk.ListPullRequestFilesOptions) ([]*forgejo_sdk.ChangedFile, error) {
	return []*forgejo_sdk.ChangedFile{{Filename: "a.go", PreviousFilename: "b.go", Status: "renamed", Additions: 1, Deletions: 2, Changes: 3, HTMLURL: "h", ContentsURL: "c", RawURL: "r"}}, s.rec("ListPullRequestFiles", o, r, i, opt)
}

func (s *stubPulls) ListPullReviews(_ context.Context, o, r string, i int64, opt forgejo_sdk.ListPullReviewsOptions) ([]*forgejo_sdk.PullReview, error) {
	return []*forgejo_sdk.PullReview{fixedReview()}, s.rec("ListPullReviews", o, r, i, opt)
}

func (s *stubPulls) GetPullReview(_ context.Context, o, r string, i, id int64) (*forgejo_sdk.PullReview, error) {
	return fixedReview(), s.rec("GetPullReview", o, r, i, id)
}

func (s *stubPulls) ListPullReviewComments(_ context.Context, o, r string, i, id int64) ([]*forgejo_sdk.PullReviewComment, error) {
	return []*forgejo_sdk.PullReviewComment{fixedReviewComment()}, s.rec("ListPullReviewComments", o, r, i, id)
}

func (s *stubPulls) CreatePullReview(_ context.Context, o, r string, i int64, opt forgejo_sdk.CreatePullReviewOptions) (*forgejo_sdk.PullReview, error) {
	return fixedReview(), s.rec("CreatePullReview", o, r, i, opt)
}

func (s *stubPulls) SubmitPullReview(_ context.Context, o, r string, i, id int64, opt forgejo_sdk.SubmitPullReviewOptions) (*forgejo_sdk.PullReview, error) {
	return fixedReview(), s.rec("SubmitPullReview", o, r, i, id, opt)
}

func (s *stubPulls) DismissPullReview(_ context.Context, o, r string, i, id int64, opt forgejo_sdk.DismissPullReviewOptions) error {
	return s.rec("DismissPullReview", o, r, i, id, opt)
}

func (s *stubPulls) DeletePullReview(_ context.Context, o, r string, i, id int64) error {
	return s.rec("DeletePullReview", o, r, i, id)
}

func (s *stubPulls) CreateReviewRequests(_ context.Context, o, r string, i int64, opt forgejo_sdk.PullReviewRequestOptions) error {
	return s.rec("CreateReviewRequests", o, r, i, opt)
}

func (s *stubPulls) DeleteReviewRequests(_ context.Context, o, r string, i int64, opt forgejo_sdk.PullReviewRequestOptions) error {
	return s.rec("DeleteReviewRequests", o, r, i, opt)
}

// invoke calls a registered tool the way the server does and renders what the
// client would see: the text, or the error, then the port calls made.
func invoke(t *testing.T, stub *stubPulls, tool string, args map[string]any) string {
	t.Helper()
	s := server.NewMCPServer("test", "0")
	f := fake.Factory{Forgejo: stub}
	RegisterTool(s, f)
	RegisterReviewTools(s, f)
	st := s.GetTool(tool)
	if st == nil {
		t.Fatalf("tool %s not registered", tool)
	}
	base := map[string]any{"owner": "octo", "repo": "hello"}
	for k, v := range args {
		if v == nil {
			delete(base, k) // a nil value means "leave the argument out"
			continue
		}
		base[k] = v
	}
	res, err := st.Handler(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Name: tool, Arguments: base}})
	var out string
	switch {
	case err != nil:
		out = "ERROR: " + err.Error()
	case res.IsError:
		out = "RESULT-ERROR: " + res.Content[0].(mcp.TextContent).Text
	default:
		out = res.Content[0].(mcp.TextContent).Text
	}
	return fmt.Sprintf("%s\n-- port calls --\n%s\n", out, strings.Join(stub.calls, "\n"))
}

type goldenCase struct {
	name string
	tool string
	args map[string]any
	stub func(*stubPulls)
}

var goldenCases = []goldenCase{
	{name: "get_pr", tool: GetPullRequestByIndexToolName, args: map[string]any{"index": float64(7)}, stub: func(s *stubPulls) { s.pr = mergedPR() }},
	{name: "get_pr_open", tool: GetPullRequestByIndexToolName, args: map[string]any{"index": float64(7)}, stub: func(s *stubPulls) { s.pr = unmergedPR() }},
	// verbose=true restores the full entity; this golden is the pre-compact output.
	{name: "get_pr_verbose", tool: GetPullRequestByIndexToolName, args: map[string]any{"index": float64(7), "verbose": true}},
	{name: "get_pr_string_index", tool: GetPullRequestByIndexToolName, args: map[string]any{"index": "7"}},
	// Forgiving argument handling of the original handlers, kept: a missing
	// required argument is forwarded as its zero value, and a fractional
	// number is truncated.
	{name: "get_pr_missing_required", tool: GetPullRequestByIndexToolName, args: map[string]any{"owner": nil, "repo": nil}},
	{name: "get_pr_fractional_index", tool: GetPullRequestByIndexToolName, args: map[string]any{"index": 5.5}},
	{name: "list_prs_fractional_paging", tool: ListRepoPullRequestsToolName, args: map[string]any{"page": 2.9, "limit": 3.5}},
	{name: "merge_missing_style", tool: MergePullRequestToolName, args: map[string]any{"index": float64(7)}},
	{name: "get_pr_port_error", tool: GetPullRequestByIndexToolName, args: map[string]any{"index": float64(7)}, stub: func(s *stubPulls) { s.fail = errors.New("boom") }},
	// A merged and an open pull request, each with head and base branches.
	{name: "list_prs_defaults", tool: ListRepoPullRequestsToolName, stub: func(s *stubPulls) {
		open := unmergedPR()
		open.Index, open.Title = 8, "open"
		s.prs = []*forgejo_sdk.PullRequest{mergedPR(), open}
	}},
	// verbose=true restores the full page; this golden is the pre-compact output.
	{name: "list_prs_verbose", tool: ListRepoPullRequestsToolName, args: map[string]any{"verbose": true}},
	{name: "list_prs_filtered", tool: ListRepoPullRequestsToolName, args: map[string]any{"state": "all", "sort": "oldest", "milestone": "4", "page": float64(2), "limit": float64(5)}},
	{name: "list_prs_bad_milestone", tool: ListRepoPullRequestsToolName, args: map[string]any{"milestone": "soon"}},
	{name: "create_pr", tool: CreatePullRequestToolName, args: map[string]any{"head": "feat", "base": "main", "title": "T", "body": "B"}},
	{name: "update_pr", tool: UpdatePullRequestToolName, args: map[string]any{"index": float64(7), "title": "T", "body": "B", "base": "dev", "assignee": "bob", "milestone": "4"}},
	{name: "create_pr_verbose", tool: CreatePullRequestToolName, args: map[string]any{"head": "feat", "base": "main", "title": "T", "body": "B", "verbose": true}},
	{name: "update_pr_verbose", tool: UpdatePullRequestToolName, args: map[string]any{"index": float64(7), "title": "T", "body": "B", "verbose": true}},
	{name: "update_pr_empty", tool: UpdatePullRequestToolName, args: map[string]any{"index": float64(7)}},
	{name: "update_pr_bad_milestone", tool: UpdatePullRequestToolName, args: map[string]any{"index": float64(7), "milestone": "x1"}},
	{name: "list_reviews_defaults", tool: ListPullReviewsToolName, args: map[string]any{"index": float64(7)}},
	{name: "list_reviews_paged", tool: ListPullReviewsToolName, args: map[string]any{"index": float64(7), "page": float64(3), "limit": float64(9)}},
	{name: "get_review", tool: GetPullReviewToolName, args: map[string]any{"index": float64(7), "id": float64(21)}},
	{name: "list_review_comments", tool: ListPullReviewCommentsToolName, args: map[string]any{"index": float64(7), "id": float64(21)}},
	{name: "merge_ok", tool: MergePullRequestToolName, args: map[string]any{"index": float64(7), "style": "squash", "title": "t", "message": "m", "delete_branch_after_merge": true, "force_merge": true},
		stub: func(s *stubPulls) { s.merge = forge.MergeResult{Merged: true, StatusCode: 200} }},
	{name: "merge_scheduled", tool: MergePullRequestToolName, args: map[string]any{"index": float64(7), "style": "merge", "merge_when_checks_succeed": true},
		stub: func(s *stubPulls) { s.merge = forge.MergeResult{StatusCode: 201} }},
	{name: "merge_declined", tool: MergePullRequestToolName, args: map[string]any{"index": float64(7), "style": "merge"},
		stub: func(s *stubPulls) { s.merge = forge.MergeResult{StatusCode: 405} }},
	{name: "merge_port_error", tool: MergePullRequestToolName, args: map[string]any{"index": float64(7), "style": "merge"}, stub: func(s *stubPulls) { s.fail = errors.New("boom") }},
	{name: "files_defaults", tool: ListPullRequestFilesToolName, args: map[string]any{"index": float64(7)}},
	{name: "files_paged", tool: ListPullRequestFilesToolName, args: map[string]any{"index": float64(7), "page": float64(2), "limit": float64(10)}},
	{name: "diff_full", tool: GetPullRequestDiffToolName, args: map[string]any{"index": float64(7)}},
	{name: "diff_file", tool: GetPullRequestDiffToolName, args: map[string]any{"index": float64(7), "file_path": "cmd/main.go"}},
	{name: "diff_file_missing", tool: GetPullRequestDiffToolName, args: map[string]any{"index": float64(7), "file_path": "nope.go"}},
	{name: "create_review", tool: CreatePullReviewToolName, args: map[string]any{"index": float64(7), "state": "COMMENT", "body": "b", "comments": `[{"path":"a.go","body":"c","new_position":2}]`}},
	{name: "create_review_bad_comments", tool: CreatePullReviewToolName, args: map[string]any{"index": float64(7), "state": "COMMENT", "comments": "{"}},
	{name: "submit_review", tool: SubmitPullReviewToolName, args: map[string]any{"index": float64(7), "id": float64(21), "state": "APPROVED", "body": "b"}},
	{name: "dismiss_review", tool: DismissPullReviewToolName, args: map[string]any{"index": float64(7), "id": float64(21), "message": "stale"}},
	{name: "delete_review", tool: DeletePullReviewToolName, args: map[string]any{"index": float64(7), "id": float64(21)}},
	{name: "create_review_requests", tool: CreateReviewRequestsToolName, args: map[string]any{"index": float64(7), "reviewers": "a, b,,c", "team_reviewers": " t1 "}},
	{name: "delete_review_requests", tool: DeleteReviewRequestsToolName, args: map[string]any{"index": float64(7), "reviewers": "a"}},
	{name: "delete_review_requests_port_error", tool: DeleteReviewRequestsToolName, args: map[string]any{"index": float64(7)}, stub: func(s *stubPulls) { s.fail = errors.New("boom") }},
}

func TestGolden(t *testing.T) {
	t.Parallel()
	for _, c := range goldenCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			stub := &stubPulls{}
			if c.stub != nil {
				c.stub(stub)
			}
			mcptest.Golden(t, c.name, []byte(invoke(t, stub, c.tool, c.args)))
		})
	}
}

// MaxResponseItems reports an unknown ceiling: the pull goldens pin the paging
// contract without a ceiling, as they always have.
func (s *stubPulls) MaxResponseItems(context.Context) (int, bool) { return 0, false }
