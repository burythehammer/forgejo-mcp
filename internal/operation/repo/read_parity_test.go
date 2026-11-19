// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"encoding/json"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/model"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/mcptest"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

// The parity tests drive a tool and its forgejo:// resource over the same fake
// data and compare the facts each returns. Where the two are meant to differ,
// the difference is named in the test and pinned by a golden.

const parityCommitSHA2 = "b4e2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0"

func repoURI() string { return "forgejo://repo/" + filesOwner + "/" + filesRepo }

// decodeRepoView reads get_repo's envelope.
func decodeRepoView(t *testing.T, res *mcp.CallToolResult) model.RepositoryView {
	t.Helper()
	var envelope struct {
		Result model.RepositoryView `json:"Result"`
	}
	if err := json.Unmarshal([]byte(extractText(t, res)), &envelope); err != nil {
		t.Fatalf("get_repo JSON: %v", err)
	}
	return envelope.Result
}

// rfc3339 matches the timestamps the fake stamps on commits and statuses at
// the moment they are seeded, which differ from run to run.
var rfc3339 = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?Z`)

// stableTimes is golden output with the seeding-time timestamps replaced, so
// the golden pins the shape and the facts, not the clock.
func stableTimes(s string) []byte {
	return []byte(rfc3339.ReplaceAllString(s, "2026-01-02T03:04:05Z"))
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

// repoFacts is what a caller learns about a repository. Both paths must agree.
type repoFacts struct {
	Owner, Name, FullName, Description, HTMLURL, DefaultBranch   string
	Fork, Archived, Private, HasIssues, HasWiki, HasPullRequests bool
	Stars, Forks, Watchers, OpenIssues, OpenPRs, Size            int
}

func repoFactsOfView(v model.RepositoryView) repoFacts {
	r := v.Repository
	owner := ""
	if r.Owner != nil {
		owner = r.Owner.UserName
	}
	return repoFacts{
		Owner: owner, Name: r.Name, FullName: r.FullName, Description: r.Description, HTMLURL: r.HTMLURL,
		DefaultBranch: r.DefaultBranch, Fork: r.Fork, Archived: r.Archived, Private: r.Private,
		HasIssues: r.HasIssues, HasWiki: r.HasWiki, HasPullRequests: r.HasPullRequests,
		Stars: r.Stars, Forks: r.Forks, Watchers: r.Watchers, OpenIssues: r.OpenIssues, OpenPRs: r.OpenPulls, Size: r.Size,
	}
}

func repoFactsOfPayload(p repoResourcePayload) repoFacts {
	return repoFacts{
		Owner: p.Owner, Name: p.Name, FullName: p.FullName, Description: p.Description, HTMLURL: p.HTMLURL,
		DefaultBranch: p.DefaultBranch, Fork: p.Fork, Archived: p.Archived, Private: p.Private,
		HasIssues: p.HasIssues, HasWiki: p.HasWiki, HasPullRequests: p.HasPullRequests,
		Stars: p.StarsCount, Forks: p.ForksCount, Watchers: p.WatchersCount,
		OpenIssues: p.OpenIssuesCount, OpenPRs: p.OpenPRCount, Size: p.Size,
	}
}

func TestRepoReadParity_ToolAndResourceAgree(t *testing.T) {
	t.Parallel()
	h, f := newFilesFixture(t)
	ctx := t.Context()
	if err := f.SetRepoTopics(ctx, filesOwner, filesRepo, []string{"go", "mcp"}); err != nil {
		t.Fatal(err)
	}

	res, err := h.GetRepoFn(ctx, filesReq(nil))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("get_repo: err=%v res=%+v", err, res)
	}
	view := decodeRepoView(t, res)

	contents, err := h.repoResourceHandler(ctx, mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: repoURI()}})
	if err != nil {
		t.Fatalf("repo resource: %v", err)
	}
	var p repoResourcePayload
	if err := json.Unmarshal([]byte(contents[0].(mcp.TextResourceContents).Text), &p); err != nil {
		t.Fatalf("repo resource JSON: %v", err)
	}

	if got, want := repoFactsOfPayload(p), repoFactsOfView(view); !reflect.DeepEqual(got, want) {
		t.Fatalf("facts differ\nresource: %+v\ntool:     %+v", got, want)
	}
}

// The get_repo tool carries topics; the resource does not. That is the
// intentional difference, pinned here and by the goldens.
func TestRepoReadParity_TopicsAreToolOnly(t *testing.T) {
	t.Parallel()
	h, f := newFilesFixture(t)
	ctx := t.Context()
	if err := f.SetRepoTopics(ctx, filesOwner, filesRepo, []string{"go", "mcp"}); err != nil {
		t.Fatal(err)
	}

	res, err := h.GetRepoFn(ctx, filesReq(nil))
	if err != nil {
		t.Fatal(err)
	}
	if got := decodeRepoView(t, res).Topics; !reflect.DeepEqual(got, []string{"go", "mcp"}) {
		t.Fatalf("get_repo topics = %v, want [go mcp]", got)
	}

	contents, err := h.repoResourceHandler(ctx, mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: repoURI()}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(contents[0].(mcp.TextResourceContents).Text, "topics") {
		t.Fatal("repo resource must not carry topics")
	}
}

func TestRepoReadGolden(t *testing.T) {
	t.Parallel()
	h, f := newFilesFixture(t)
	ctx := t.Context()
	if err := f.SetRepoTopics(ctx, filesOwner, filesRepo, []string{"go", "mcp"}); err != nil {
		t.Fatal(err)
	}

	res, err := h.GetRepoFn(ctx, filesReq(nil))
	if err != nil {
		t.Fatal(err)
	}
	mcptest.Golden(t, "read_repo_tool", stableTimes(extractText(t, res)))

	contents, err := h.repoResourceHandler(ctx, mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: repoURI()}})
	if err != nil {
		t.Fatal(err)
	}
	mcptest.Golden(t, "read_repo_resource", stableTimes(contents[0].(mcp.TextResourceContents).Text))
}

// seedParityCommits makes two commits on the fixture repository.
func seedParityCommits(t *testing.T, f *fake.Forgejo) {
	t.Helper()
	if err := f.AddCommit(filesOwner, filesRepo, testSHA, "fix: resolve race condition\n\nDetailed explanation.", "a.go"); err != nil {
		t.Fatal(err)
	}
	if err := f.AddCommit(filesOwner, filesRepo, parityCommitSHA2, "feat: second", "b.go"); err != nil {
		t.Fatal(err)
	}
}

// The forgejo://…/commit/{sha} resource reads one commit; list_repo_commits
// reads the same commits in pages. For the same SHA both must return the same
// commit.
func TestCommitReadParity_ResourceMatchesListEntry(t *testing.T) {
	t.Parallel()
	h, f := newFilesFixture(t)
	ctx := t.Context()
	seedParityCommits(t, f)

	res, err := h.ListRepoCommitsFn(ctx, filesReq(map[string]any{"page": float64(1), "limit": float64(100)}))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("list_repo_commits: err=%v res=%+v", err, res)
	}
	var envelope struct {
		Result model.CommitPage `json:"Result"`
	}
	if err := json.Unmarshal([]byte(extractText(t, res)), &envelope); err != nil {
		t.Fatalf("list_repo_commits JSON: %v", err)
	}
	listed := envelope.Result.Commits
	var fromList *model.Commit
	for i := range listed {
		if listed[i].SHA == parityCommitSHA2 {
			fromList = listed[i]
		}
	}
	if fromList == nil {
		t.Fatalf("list_repo_commits has no %s", parityCommitSHA2)
	}

	contents, err := h.commitResourceHandler(ctx, makeCommitResourceRequest(parityCommitSHA2))
	if err != nil {
		t.Fatalf("commit resource: %v", err)
	}
	var fromResource model.Commit
	if err := json.Unmarshal([]byte(contents[0].(mcp.TextResourceContents).Text), &fromResource); err != nil {
		t.Fatalf("commit resource JSON: %v", err)
	}
	if fromResource.SHA != fromList.SHA || fromResource.RepoCommit == nil || fromList.RepoCommit == nil ||
		fromResource.RepoCommit.Message != fromList.RepoCommit.Message {
		t.Fatalf("commit differs\nresource: %+v\nlist:     %+v", fromResource.RepoCommit, fromList.RepoCommit)
	}
}

func TestCommitReadGolden(t *testing.T) {
	t.Parallel()
	h, f := newFilesFixture(t)
	seedParityCommits(t, f)

	contents, err := h.commitResourceHandler(t.Context(), makeCommitResourceRequest(testSHA))
	if err != nil {
		t.Fatal(err)
	}
	mcptest.Golden(t, "read_commit_resource", stableTimes(contents[0].(mcp.TextResourceContents).Text))
}

func TestStatusReadParity_ToolAndResourceAgree(t *testing.T) {
	t.Parallel()
	h, f := newFilesFixture(t)
	knownCommit(t, f)
	seedStatuses(t, f, forgejo_sdk.StatusSuccess, forgejo_sdk.StatusFailure, forgejo_sdk.StatusPending)
	ctx := t.Context()

	res, err := h.GetCommitStatusesFn(ctx, statusesReq(nil))
	if err != nil || res == nil || res.IsError {
		t.Fatalf("get_commit_statuses: err=%v res=%+v", err, res)
	}
	tool := decodeStatuses(t, res)

	contents, err := h.statusResourceHandler(ctx, makeStatusResourceRequest(testSHA))
	if err != nil {
		t.Fatalf("status resource: %v", err)
	}
	p := statusPayload(t, contents)

	// Compared as JSON: the resource and the tool carry the same wire shape
	// under two Go types.
	if got, want := mustJSON(t, p.Statuses), mustJSON(t, tool.Statuses); got != want {
		t.Fatalf("statuses differ\nresource: %s\ntool:     %s", got, want)
	}
	if p.TotalCount != len(tool.Statuses) || (tool.TotalCount != nil && *tool.TotalCount != len(tool.Statuses)) {
		t.Fatalf("totals differ: resource %d, tool %v", p.TotalCount, tool.TotalCount)
	}
	if p.State != "failure" {
		t.Fatalf("state = %q, want failure", p.State)
	}
	if p.Truncated {
		t.Fatal("three statuses must not truncate")
	}
}

// Over the embedded cap the two paths differ on purpose. The resource shows 30
// statuses, marks itself truncated and names the tool. Both report Forgejo's
// grand total; the tool shows the page asked for, the resource the capped list.
func TestStatusReadParity_OverCapDiffersOnPurpose(t *testing.T) {
	t.Parallel()
	h, f := newFilesFixture(t)
	knownCommit(t, f)
	states := make([]forgejo_sdk.StatusState, 40)
	for i := range states {
		states[i] = forgejo_sdk.StatusSuccess
	}
	seedStatuses(t, f, states...)
	ctx := t.Context()

	res, err := h.GetCommitStatusesFn(ctx, statusesReq(nil))
	if err != nil {
		t.Fatal(err)
	}
	tool := decodeStatuses(t, res)
	if len(tool.Statuses) != 30 || tool.TotalCount == nil || *tool.TotalCount != 40 {
		t.Fatalf("tool: %d statuses, total %v; want 30 and 40", len(tool.Statuses), tool.TotalCount)
	}

	contents, err := h.statusResourceHandler(ctx, makeStatusResourceRequest(testSHA))
	if err != nil {
		t.Fatal(err)
	}
	p := statusPayload(t, contents)
	if len(p.Statuses) != 30 || !p.Truncated || p.ListTool != "get_commit_statuses" || p.TotalCount != 40 {
		t.Fatalf("resource: %d statuses, truncated %v, list_tool %q, total_count %d; want 30, true, get_commit_statuses, 40",
			len(p.Statuses), p.Truncated, p.ListTool, p.TotalCount)
	}
	mcptest.Golden(t, "read_status_resource_over_cap", stableTimes(contents[0].(mcp.TextResourceContents).Text))
	mcptest.Golden(t, "read_status_tool_over_cap", stableTimes(extractText(t, res)))
}

// The combined state is read over every page: the one failure sits on the
// second page of statuses (page size 50), and the golden pins the answer.
func TestStatusReadGolden_FailureOnASecondPageReachesTheState(t *testing.T) {
	t.Parallel()
	h, f := newFilesFixture(t)
	knownCommit(t, f)
	states := make([]forgejo_sdk.StatusState, 60)
	for i := range states {
		states[i] = forgejo_sdk.StatusSuccess
	}
	states[55] = forgejo_sdk.StatusFailure
	seedStatuses(t, f, states...)

	contents, err := h.statusResourceHandler(t.Context(), makeStatusResourceRequest(testSHA))
	if err != nil {
		t.Fatal(err)
	}
	p := statusPayload(t, contents)
	if p.State != "failure" || p.TotalCount != 60 {
		t.Fatalf("state %q total %d, want failure and 60", p.State, p.TotalCount)
	}
	mcptest.Golden(t, "read_status_resource_second_page_failure", stableTimes(contents[0].(mcp.TextResourceContents).Text))
}

// The last part of a message longer than two parts. The golden pins the commit
// JSON and the markdown sidecar a caller reads to finish the message.
func TestCommitMessagePartGolden_LastPart(t *testing.T) {
	t.Parallel()
	h, f := newFilesFixture(t)
	msg := "subject\n\n" + strings.Repeat("m", 2*commitMessageCap+100)
	if err := f.AddCommit(filesOwner, filesRepo, testSHA, msg, "a.go"); err != nil {
		t.Fatal(err)
	}
	uri := commitURI(testSHA, "message_page=3")
	contents, err := h.commitResourceHandler(t.Context(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: uri}})
	if err != nil {
		t.Fatal(err)
	}
	if len(contents) != 2 {
		t.Fatalf("content blocks = %d, want JSON and markdown", len(contents))
	}
	mcptest.Golden(t, "read_commit_message_last_part", stableTimes(contents[0].(mcp.TextResourceContents).Text+"\n"+contents[1].(mcp.TextResourceContents).Text))
}
