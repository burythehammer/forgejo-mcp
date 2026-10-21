// SPDX-License-Identifier: GPL-3.0-or-later

package operation

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"time"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/actions"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/attachment"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/branchprotection"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/hook"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/issue"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcptest"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/org"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/packages"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/pull"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/release"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/repo"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/search"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/tracking"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/user"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/wiki"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// These goldens pin the pagination contract of every list and search tool: its
// default paging, an explicit page and limit, and the last page. Each tool runs
// through the server's own dispatch against the fake Forgejo, seeded with five
// items at fixed times, so the bytes are deterministic and the page windows are
// observable: page 1 at limit 2 names items 1 and 2, page 2 names 3 and 4, and
// page 3 names the last item alone. Where a tool does not paginate, the goldens
// show that too. Inconsistencies between tools are pinned, not corrected.

const (
	pageOwner = "octo"
	pageRepo  = "hello"
	pageItems = 5
)

var pageNow = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

// pageCase is one call shape. Explicit and last both use limit 2, so the last
// page is short (one item) and page 2 is a full middle window.
var pageCases = []struct {
	name  string
	extra map[string]any
}{
	{"default", nil},
	{"explicit", map[string]any{"page": float64(2), "limit": float64(2)}},
	{"last", map[string]any{"page": float64(3), "limit": float64(2)}},
}

// pageTool describes one list tool: the arguments that identify the collection
// (never paging) and a seed that fills the fake with pageItems entries.
type pageTool struct {
	tool string
	args map[string]any
	seed func(t *testing.T, f *fake.Forgejo)
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func repoArgs(extra map[string]any) map[string]any {
	m := map[string]any{"owner": pageOwner, "repo": pageRepo}
	for k, v := range extra {
		m[k] = v
	}
	return m
}

func seedRepo(t *testing.T, f *fake.Forgejo) {
	t.Helper()
	f.AddRepo(pageOwner, pageRepo)
}

// seedIssues creates pageItems issues, numbered 1..5.
func seedIssues(t *testing.T, f *fake.Forgejo) {
	t.Helper()
	seedRepo(t, f)
	for i := 1; i <= pageItems; i++ {
		_, err := f.CreateIssue(context.Background(), pageOwner, pageRepo, forgejo_sdk.CreateIssueOption{Title: fmt.Sprintf("issue %d", i)})
		must(t, err)
	}
}

// seedIssueWithComments creates issue 1 and pageItems comments on it.
func seedIssueWithComments(t *testing.T, f *fake.Forgejo) {
	t.Helper()
	seedIssues(t, f)
	for i := 1; i <= pageItems; i++ {
		_, err := f.CreateIssueComment(context.Background(), pageOwner, pageRepo, 1, fmt.Sprintf("comment %d", i))
		must(t, err)
	}
}

// seedPulls creates a head branch and pageItems pull requests, numbered 1..5.
func seedPulls(t *testing.T, f *fake.Forgejo) {
	t.Helper()
	seedRepo(t, f)
	ctx := context.Background()
	for i := 1; i <= pageItems; i++ {
		_, err := f.CreateBranch(ctx, pageOwner, pageRepo, forgejo_sdk.CreateBranchOption{BranchName: fmt.Sprintf("feat-%d", i), OldBranchName: "main"})
		must(t, err)
		_, err = f.CreatePullRequest(ctx, pageOwner, pageRepo, forgejo_sdk.CreatePullRequestOption{Head: fmt.Sprintf("feat-%d", i), Base: "main", Title: fmt.Sprintf("pr %d", i)})
		must(t, err)
	}
}

// seedReleases creates pageItems releases, IDs 1..5 in creation order.
func seedReleases(t *testing.T, f *fake.Forgejo) {
	t.Helper()
	seedRepo(t, f)
	for i := 1; i <= pageItems; i++ {
		_, err := f.CreateRelease(context.Background(), pageOwner, pageRepo, forgejo_sdk.CreateReleaseOption{TagName: fmt.Sprintf("v1.0.%d", i), Title: fmt.Sprintf("release %d", i)})
		must(t, err)
	}
}

// seedRunJobsAndArtifacts creates workflow ci.yml and run 1 with pageItems jobs and
// artifacts, which the run-scoped list tools read.
func seedRunJobsAndArtifacts(t *testing.T, f *fake.Forgejo) {
	t.Helper()
	seedRepo(t, f)
	f.AddWorkflow(pageOwner, pageRepo, "ci.yml")
	run := f.AddWorkflowRun(pageOwner, pageRepo, forgejo_sdk.ActionRun{Title: "ci", Status: "success"})
	for i := 1; i <= pageItems; i++ {
		_, err := f.AddRunJob(pageOwner, pageRepo, run.ID, forge.ActionRunJob{Name: fmt.Sprintf("job %d", i), Status: "success"})
		must(t, err)
		_, err = f.AddRunArtifact(pageOwner, pageRepo, run.ID, forge.ActionArtifact{Name: fmt.Sprintf("artifact-%d", i), SizeInBytes: 10})
		must(t, err)
	}
}

// pageToolsTable is every list and search tool registered by RegisterTool.
var pageToolsTable = []pageTool{
	{tool: issue.ListRepoIssuesToolName, args: repoArgs(nil), seed: seedIssues},
	{tool: issue.ListIssueCommentsToolName, args: repoArgs(map[string]any{"index": float64(1)}), seed: seedIssueWithComments},
	{tool: issue.ListRepoMilestonesToolName, args: repoArgs(nil), seed: func(t *testing.T, f *fake.Forgejo) {
		seedRepo(t, f)
		for i := 1; i <= pageItems; i++ {
			f.SeedMilestone(pageOwner, pageRepo, fmt.Sprintf("milestone %d", i))
		}
	}},
	{tool: issue.ListRepoLabelsToolName, args: repoArgs(nil), seed: func(t *testing.T, f *fake.Forgejo) {
		seedRepo(t, f)
		for i := 1; i <= pageItems; i++ {
			_, err := f.CreateRepoLabel(context.Background(), pageOwner, pageRepo, forgejo_sdk.CreateLabelOption{Name: fmt.Sprintf("label %d", i), Color: "00ff00"})
			must(t, err)
		}
	}},
	{tool: issue.ListOrgLabelsToolName, args: map[string]any{"org": "acme"}, seed: func(t *testing.T, f *fake.Forgejo) {
		for i := 1; i <= pageItems; i++ {
			seedOrg(t, f, "acme")
			_, err := f.CreateOrgLabel(context.Background(), "acme", forgejo_sdk.CreateLabelOption{Name: fmt.Sprintf("org label %d", i), Color: "0000ff"})
			must(t, err)
		}
	}},
	{tool: issue.SearchIssuesToolName, args: map[string]any{"owner": pageOwner}, seed: seedIssues},
	{tool: issue.ListIssueDependenciesToolName, args: repoArgs(map[string]any{"index": float64(1)}), seed: func(t *testing.T, f *fake.Forgejo) {
		seedIssues(t, f)
		for i := 2; i <= pageItems; i++ {
			// Issue 1 depends on issues 2..5.
			must(t, f.AddIssueDependency(context.Background(), pageOwner, pageRepo, 1, forge.IssueRef{Owner: pageOwner, Repo: pageRepo, Index: int64(i)}))
		}
	}},
	{tool: issue.ListIssueDependentsToolName, args: repoArgs(map[string]any{"index": float64(1)}), seed: func(t *testing.T, f *fake.Forgejo) {
		seedIssues(t, f)
		for i := 2; i <= pageItems; i++ {
			// Issues 2..5 depend on issue 1.
			must(t, f.AddIssueDependency(context.Background(), pageOwner, pageRepo, int64(i), forge.IssueRef{Owner: pageOwner, Repo: pageRepo, Index: 1}))
		}
	}},
	{tool: tracking.ListIssueTrackedTimesToolName, args: repoArgs(map[string]any{"index": float64(1)}), seed: seedTrackedTimes},
	{tool: pull.ListRepoPullRequestsToolName, args: repoArgs(nil), seed: seedPulls},
	{tool: pull.ListPullReviewsToolName, args: repoArgs(map[string]any{"index": float64(1)}), seed: func(t *testing.T, f *fake.Forgejo) {
		seedPulls(t, f)
		for i := 1; i <= pageItems; i++ {
			_, err := f.CreatePullReview(context.Background(), pageOwner, pageRepo, 1, forgejo_sdk.CreatePullReviewOptions{State: forgejo_sdk.ReviewStateComment, Body: fmt.Sprintf("review %d", i)})
			must(t, err)
		}
	}},
	{tool: pull.ListPullReviewCommentsToolName, args: repoArgs(map[string]any{"index": float64(1), "id": float64(1)}), seed: func(t *testing.T, f *fake.Forgejo) {
		seedPulls(t, f)
		// One review carries the inline comments; review 1 is its ID.
		var comments []forgejo_sdk.CreatePullReviewComment
		for i := 1; i <= pageItems; i++ {
			comments = append(comments, forgejo_sdk.CreatePullReviewComment{Path: fmt.Sprintf("file-%d.txt", i), Body: fmt.Sprintf("line note %d", i), NewLineNum: int64(i)})
		}
		_, err := f.CreatePullReview(context.Background(), pageOwner, pageRepo, 1, forgejo_sdk.CreatePullReviewOptions{State: forgejo_sdk.ReviewStateComment, Body: "review", Comments: comments})
		must(t, err)
	}},
	{tool: pull.ListPullRequestFilesToolName, args: repoArgs(map[string]any{"index": float64(1)}), seed: func(t *testing.T, f *fake.Forgejo) {
		seedPulls(t, f)
		var files []*forgejo_sdk.ChangedFile
		for i := 1; i <= pageItems; i++ {
			files = append(files, &forgejo_sdk.ChangedFile{Filename: fmt.Sprintf("file-%d.txt", i), Status: "modified"})
		}
		must(t, f.SetPullRequestFiles(pageOwner, pageRepo, 1, files...))
	}},
	{tool: release.ListReleasesToolName, args: repoArgs(nil), seed: seedReleases},
	{tool: release.ListReleaseAttachmentsToolName, args: repoArgs(map[string]any{"release_id": float64(1)}), seed: func(t *testing.T, f *fake.Forgejo) {
		seedReleases(t, f)
		for i := 1; i <= pageItems; i++ {
			_, err := f.CreateReleaseAttachment(context.Background(), pageOwner, pageRepo, 1, forge.Upload{Filename: fmt.Sprintf("asset-%d.txt", i), MIMEType: "text/plain", Content: strings.NewReader("x")})
			must(t, err)
		}
	}},
	{tool: actions.ListWorkflowRunsToolName, args: repoArgs(nil), seed: func(t *testing.T, f *fake.Forgejo) {
		seedRepo(t, f)
		f.AddWorkflow(pageOwner, pageRepo, "ci.yml")
		for i := 1; i <= pageItems; i++ {
			f.AddWorkflowRun(pageOwner, pageRepo, forgejo_sdk.ActionRun{Title: fmt.Sprintf("run %d", i), Status: "success"})
		}
	}},
	{tool: actions.ListActionRunJobsToolName, args: repoArgs(map[string]any{"run_id": float64(1)}), seed: seedRunJobsAndArtifacts},
	{tool: actions.ListActionRunArtifactsToolName, args: repoArgs(map[string]any{"run_id": float64(1)}), seed: seedRunJobsAndArtifacts},
	{tool: actions.ListRepoActionSecretsToolName, args: repoArgs(nil), seed: func(t *testing.T, f *fake.Forgejo) {
		seedRepo(t, f)
		for i := 1; i <= pageItems; i++ {
			must(t, f.SetRepoActionSecret(context.Background(), pageOwner, pageRepo, forgejo_sdk.CreateSecretOption{Name: fmt.Sprintf("SECRET_%d", i), Data: "v"}))
		}
	}},
	{tool: actions.ListOrgActionSecretsToolName, args: map[string]any{"org": "acme"}, seed: func(t *testing.T, f *fake.Forgejo) {
		f.AddActionsOrg("acme")
		for i := 1; i <= pageItems; i++ {
			must(t, f.SetOrgActionSecret(context.Background(), "acme", forgejo_sdk.CreateSecretOption{Name: fmt.Sprintf("ORG_SECRET_%d", i), Data: "v"}))
		}
	}},
	{tool: wiki.ListWikiPagesToolName, args: repoArgs(nil), seed: func(t *testing.T, f *fake.Forgejo) {
		seedRepo(t, f)
		for i := 1; i <= pageItems; i++ {
			_, err := f.CreateWikiPage(context.Background(), pageOwner, pageRepo, fmt.Sprintf("Page %d", i), base64.StdEncoding.EncodeToString([]byte("body")), "create")
			must(t, err)
		}
	}},
	{tool: branchprotection.ListBranchProtectionsToolName, args: repoArgs(nil), seed: func(t *testing.T, f *fake.Forgejo) {
		seedRepo(t, f)
		for i := 1; i <= pageItems; i++ {
			_, err := f.CreateBranchProtection(context.Background(), pageOwner, pageRepo, forgejo_sdk.CreateBranchProtectionOption{RuleName: fmt.Sprintf("rule-%d", i)})
			must(t, err)
		}
	}},
	{tool: hook.ListRepoHooksToolName, args: repoArgs(nil), seed: func(t *testing.T, f *fake.Forgejo) {
		seedRepo(t, f)
		for i := 1; i <= pageItems; i++ {
			_, err := f.CreateRepoHook(context.Background(), pageOwner, pageRepo, forgejo_sdk.CreateHookOption{Type: "gitea", Config: map[string]string{"content_type": "json", "url": fmt.Sprintf("https://hooks.example/%d", i)}, Events: []string{"push"}, Active: true})
			must(t, err)
		}
	}},
	{tool: repo.ListRepoCommitsToolName, args: repoArgs(nil), seed: func(t *testing.T, f *fake.Forgejo) {
		seedRepo(t, f)
		for i := 1; i <= pageItems; i++ {
			must(t, f.AddCommit(pageOwner, pageRepo, fmt.Sprintf("%040x", i), fmt.Sprintf("commit %d", i), fmt.Sprintf("file-%d.txt", i)))
		}
	}},
	{tool: repo.ListRepoContentsToolName, args: repoArgs(map[string]any{"ref": "main", "path": "docs"}), seed: func(t *testing.T, f *fake.Forgejo) {
		seedRepo(t, f)
		for i := 1; i <= pageItems; i++ {
			must(t, f.SetFile(pageOwner, pageRepo, "main", fmt.Sprintf("docs/file-%d.md", i), "content"))
		}
	}},
	{tool: repo.ListBranchesToolName, args: repoArgs(nil), seed: func(t *testing.T, f *fake.Forgejo) {
		seedRepo(t, f)
		for i := 1; i <= pageItems; i++ {
			_, err := f.CreateBranch(context.Background(), pageOwner, pageRepo, forgejo_sdk.CreateBranchOption{BranchName: fmt.Sprintf("branch-%d", i), OldBranchName: "main"})
			must(t, err)
		}
	}},
	{tool: repo.ListRepoTopicsToolName, args: repoArgs(nil), seed: func(t *testing.T, f *fake.Forgejo) {
		seedRepo(t, f)
		for i := 1; i <= pageItems; i++ {
			must(t, f.AddRepoTopic(context.Background(), pageOwner, pageRepo, fmt.Sprintf("topic-%d", i)))
		}
	}},
	{tool: repo.ListMyReposToolName, args: nil, seed: func(t *testing.T, f *fake.Forgejo) {
		for i := 1; i <= pageItems; i++ {
			f.AddRepo(pageOwner, fmt.Sprintf("repo-%d", i))
		}
	}},
	{tool: org.ListMyOrgsToolName, args: nil, seed: func(t *testing.T, f *fake.Forgejo) {
		for i := 1; i <= pageItems; i++ {
			must(t, f.AddOrgMember(seedOrg(t, f, fmt.Sprintf("org-%d", i)), "octo"))
		}
	}},
	{tool: org.ListUserOrgsToolName, args: map[string]any{"user": "octo"}, seed: func(t *testing.T, f *fake.Forgejo) {
		for i := 1; i <= pageItems; i++ {
			must(t, f.AddOrgMember(seedOrg(t, f, fmt.Sprintf("org-%d", i)), "octo"))
		}
	}},
	{tool: org.ListOrgMembersToolName, args: map[string]any{"org": "acme"}, seed: func(t *testing.T, f *fake.Forgejo) {
		for i := 1; i <= pageItems; i++ {
			must(t, f.AddOrgMember(seedOrg(t, f, "acme"), fmt.Sprintf("member-%d", i)))
		}
	}},
	{tool: org.ListOrgTeamsToolName, args: map[string]any{"org": "acme"}, seed: func(t *testing.T, f *fake.Forgejo) {
		for i := 1; i <= pageItems; i++ {
			seedOrg(t, f, "acme")
			f.SeedTeam("acme", forgejo_sdk.Team{ID: int64(i), Name: fmt.Sprintf("team-%d", i), Permission: "read"})
		}
	}},
	{tool: search.SearchOrgTeamsToolName, args: map[string]any{"org": "acme"}, seed: func(t *testing.T, f *fake.Forgejo) {
		for i := 1; i <= pageItems; i++ {
			seedOrg(t, f, "acme")
			f.SeedTeam("acme", forgejo_sdk.Team{ID: int64(i), Name: fmt.Sprintf("team-%d", i), Permission: "read"})
		}
	}},
	{tool: search.SearchUsersToolName, args: nil, seed: func(t *testing.T, f *fake.Forgejo) {
		for i := 1; i <= pageItems; i++ {
			f.SeedUser(forgejo_sdk.User{ID: int64(i), UserName: fmt.Sprintf("user-%d", i)})
		}
	}},
	{tool: search.SearchReposToolName, args: nil, seed: func(t *testing.T, f *fake.Forgejo) {
		for i := 1; i <= pageItems; i++ {
			f.SeedSearchableRepo(forgejo_sdk.Repository{ID: int64(i), Name: fmt.Sprintf("repo-%d", i), FullName: fmt.Sprintf("octo/repo-%d", i), Owner: &forgejo_sdk.User{UserName: pageOwner}})
		}
	}},
	{tool: tracking.ListMyTrackedTimesToolName, args: nil, seed: seedTrackedTimes},
	{tool: tracking.ListRepoTrackedTimesToolName, args: repoArgs(nil), seed: seedTrackedTimes},
	{tool: tracking.ListMyStopwatchesToolName, args: nil, seed: func(t *testing.T, f *fake.Forgejo) {
		seedIssues(t, f)
		for i := 1; i <= pageItems; i++ {
			f.AddTrackableIssue(pageOwner, pageRepo, int64(i), fmt.Sprintf("issue %d", i))
			must(t, f.StartIssueStopwatch(context.Background(), pageOwner, pageRepo, int64(i)))
		}
	}},
	{tool: user.ListRepoNotificationsToolName, args: repoArgs(nil), seed: func(t *testing.T, f *fake.Forgejo) {
		seedRepo(t, f)
		for i := 1; i <= pageItems; i++ {
			f.AddNotification(pageOwner, pageRepo, fmt.Sprintf("notification %d", i), true)
		}
	}},
	{tool: attachment.ListIssueAttachmentsToolName, args: repoArgs(map[string]any{"index": float64(1)}), seed: func(t *testing.T, f *fake.Forgejo) {
		seedIssues(t, f)
		f.AddAttachmentIssue(pageOwner, pageRepo, 1)
		for i := 1; i <= pageItems; i++ {
			_, err := f.CreateIssueAttachment(context.Background(), pageOwner, pageRepo, 1, forge.Upload{Filename: fmt.Sprintf("log-%d.txt", i), MIMEType: "text/plain", Content: strings.NewReader("x")})
			must(t, err)
		}
	}},
	{tool: attachment.ListCommentAttachmentsToolName, args: repoArgs(map[string]any{"comment_id": float64(1)}), seed: func(t *testing.T, f *fake.Forgejo) {
		seedIssues(t, f)
		c, err := f.CreateIssueComment(context.Background(), pageOwner, pageRepo, 1, "comment")
		must(t, err)
		f.AddAttachmentComment(pageOwner, pageRepo, c.ID)
		for i := 1; i <= pageItems; i++ {
			_, err := f.CreateCommentAttachment(context.Background(), pageOwner, pageRepo, c.ID, forge.Upload{Filename: fmt.Sprintf("shot-%d.txt", i), MIMEType: "text/plain", Content: strings.NewReader("x")})
			must(t, err)
		}
	}},
	{tool: packages.ListPackagesToolName, args: map[string]any{"owner": pageOwner}, seed: func(t *testing.T, f *fake.Forgejo) {
		f.AddPackageOwner(pageOwner)
		for i := 1; i <= pageItems; i++ {
			f.AddPackage(pageOwner, forge.PackageVersion{Type: "generic", Name: fmt.Sprintf("pkg-%d", i), Version: "1.0.0"})
		}
	}},
	{tool: packages.ListPackageFilesToolName, args: map[string]any{"owner": pageOwner, "type": "generic", "name": "pkg", "version": "1.0.0"}, seed: func(t *testing.T, f *fake.Forgejo) {
		f.AddPackageOwner(pageOwner)
		var files []forge.PackageFile
		for i := 1; i <= pageItems; i++ {
			files = append(files, forge.PackageFile{Name: fmt.Sprintf("file-%d.bin", i), Size: 10})
		}
		f.AddPackage(pageOwner, forge.PackageVersion{Type: "generic", Name: "pkg", Version: "1.0.0"}, files...)
	}},
}

// seedOrg creates the organization name once, so several seeds can share it,
// and returns the name.
func seedOrg(t *testing.T, f *fake.Forgejo, name string) string {
	t.Helper()
	ctx := context.Background()
	if _, err := f.GetOrg(ctx, name); err == nil {
		return name
	}
	_, err := f.CreateOrg(ctx, forgejo_sdk.CreateOrgOption{Name: name})
	must(t, err)
	return name
}

// seedTrackedTimes gives issue 1 pageItems tracked-time entries.
func seedTrackedTimes(t *testing.T, f *fake.Forgejo) {
	t.Helper()
	seedIssues(t, f)
	f.AddTrackableIssue(pageOwner, pageRepo, 1, "issue 1")
	for i := 1; i <= pageItems; i++ {
		_, err := f.AddIssueTime(context.Background(), pageOwner, pageRepo, 1, forgejo_sdk.AddTimeOption{Time: int64(60 * i)})
		must(t, err)
	}
}

// TestPaginationCoversEveryListTool fails when a list or search tool is
// registered without a row in pageToolsTable, so a new list tool cannot skip
// the pagination contract.
func TestPaginationCoversEveryListTool(t *testing.T) {
	t.Parallel()
	s := server.NewMCPServer("test", "0")
	RegisterTool(s, "test", fake.Factory{Forgejo: fake.New()})
	covered := map[string]bool{}
	for _, pt := range pageToolsTable {
		covered[pt.tool] = true
	}
	for name := range s.ListTools() {
		if !strings.HasPrefix(name, "list_") && !strings.HasPrefix(name, "search_") {
			continue
		}
		if !covered[name] {
			t.Errorf("list or search tool %q has no pagination golden row", name)
		}
	}
}

// TestPaginationGoldens renders every list tool at the three call shapes.
// A new list tool added to RegisterTool without a row here is caught by
// TestPaginationCoversEveryListTool.
func TestPaginationGoldens(t *testing.T) {
	t.Parallel()
	for _, pt := range pageToolsTable {
		for _, c := range pageCases {
			t.Run(pt.tool+"/"+c.name, func(t *testing.T) {
				t.Parallel()
				mcptest.Golden(t, "page_"+pt.tool+"_"+c.name, []byte(pageInvoke(t, pt, c.extra)))
			})
		}
	}
}

// pageInvoke seeds a fresh fake, dispatches the tool through a registered
// server, and returns its output followed by the port calls it made.
func pageInvoke(t *testing.T, pt pageTool, extra map[string]any) string {
	t.Helper()
	f := fake.New()
	f.Now = func() time.Time { return pageNow }
	pt.seed(t, f)
	s := server.NewMCPServer("test", "0")
	RegisterTool(s, "test", fake.Factory{Forgejo: f})
	st := s.GetTool(pt.tool)
	if st == nil {
		t.Fatalf("tool %s not registered", pt.tool)
	}
	args := map[string]any{}
	for k, v := range pt.args {
		args[k] = v
	}
	for k, v := range extra {
		args[k] = v
	}
	res, err := st.Handler(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Name: pt.tool, Arguments: args}})
	switch {
	case err != nil:
		return "ERROR: " + err.Error() + "\n"
	case res.IsError:
		return "RESULT-ERROR: " + res.Content[0].(mcp.TextContent).Text + "\n"
	default:
		return res.Content[0].(mcp.TextContent).Text + "\n"
	}
}

// TestPaginationCeilingGolden pins the instance ceiling where a list tool
// applies it: search_issues refuses a limit above max_response_items and names
// the ceiling in the error. The other list tools do not consult the ceiling.
func TestPaginationCeilingGolden(t *testing.T) {
	t.Parallel()
	pt := pageTool{tool: issue.SearchIssuesToolName, args: map[string]any{"owner": pageOwner}, seed: func(t *testing.T, f *fake.Forgejo) {
		seedIssues(t, f)
		f.SetMaxResponseItems(2)
	}}
	mcptest.Golden(t, "page_search_issues_over_ceiling", []byte(pageInvoke(t, pt, map[string]any{"limit": float64(3)})))
}
