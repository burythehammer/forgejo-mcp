// SPDX-License-Identifier: GPL-3.0-or-later

package pull

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

// create_pull_request takes optional labels and assignees. These tests run the
// tool's handler against the fake and read the stored pull request back, so
// they assert what Forgejo would hold, not the request order.

// createPR runs create_pull_request with args over the fixture's defaults. A
// failed create comes back either as an error result or as a Go error, which
// the MCP server reports as a tool error; the caller judges both.
func createPR(h handlers, args map[string]any) (*mcp.CallToolResult, error) {
	base := map[string]any{"head": "feature", "base": "main", "title": "T"}
	for k, v := range args {
		base[k] = v
	}
	return h.CreatePullRequestFn(context.Background(), makeReq(base))
}

// mustCreatePR runs createPR and fails the test unless the create succeeded.
func mustCreatePR(t *testing.T, h handlers, args map[string]any) {
	t.Helper()
	res, err := createPR(h, args)
	if err != nil || res == nil || res.IsError {
		t.Fatalf("create failed: err=%v res=%+v", err, res)
	}
}

// listAll returns every pull request in the fixture repository, of any state.
func listAll(t *testing.T, f *fake.Forgejo) []*forgejo_sdk.PullRequest {
	t.Helper()
	prs, err := f.ListPullRequests(context.Background(), testOwner, testRepo, forgejo_sdk.ListPullRequestsOptions{State: forgejo_sdk.StateAll})
	if err != nil {
		t.Fatal(err)
	}
	return prs
}

// onlyPR returns the single pull request the fixture repository holds.
func onlyPR(t *testing.T, f *fake.Forgejo) *forgejo_sdk.PullRequest {
	t.Helper()
	prs := listAll(t, f)
	if len(prs) != 1 {
		t.Fatalf("got %d pull requests, want 1", len(prs))
	}
	return prs[0]
}

func labelNames(pr *forgejo_sdk.PullRequest) []string {
	out := make([]string, 0, len(pr.Labels))
	for _, l := range pr.Labels {
		out = append(out, l.Name)
	}
	sort.Strings(out)
	return out
}

func assigneeNames(pr *forgejo_sdk.PullRequest) []string {
	out := make([]string, 0, len(pr.Assignees))
	for _, a := range pr.Assignees {
		out = append(out, a.UserName)
	}
	sort.Strings(out)
	return out
}

func sameStrings(a, b []string) bool { return fmt.Sprint(a) == fmt.Sprint(b) }

func TestCreatePullRequestLabelsByNameIDAndMixed(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		// labels is built from the seeded label IDs: bug and docs.
		labels func(bug, docs int64) string
		want   []string
	}{
		{"by name", func(_, _ int64) string { return "bug" }, []string{"bug"}},
		{"by id", func(bug, _ int64) string { return fmt.Sprint(bug) }, []string{"bug"}},
		{"mixed name and id", func(bug, docs int64) string { return fmt.Sprintf("bug,%d", docs) }, []string{"bug", "docs"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h, f := newFixture(t)
			bug := f.SeedIssueLabel(testOwner, testRepo, "bug")
			docs := f.SeedIssueLabel(testOwner, testRepo, "docs")
			mustCreatePR(t, h, map[string]any{"labels": tc.labels(bug, docs)})
			if got := labelNames(onlyPR(t, f)); !sameStrings(got, tc.want) {
				t.Fatalf("stored labels %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCreatePullRequestAssigneesAreStored(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	mustCreatePR(t, h, map[string]any{"assignees": "octo,hubot"})
	if got, want := assigneeNames(onlyPR(t, f)), []string{"hubot", "octo"}; !sameStrings(got, want) {
		t.Fatalf("stored assignees %v, want %v", got, want)
	}
}

func TestCreatePullRequestLabelsAndAssigneesTogether(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	f.SeedIssueLabel(testOwner, testRepo, "bug")
	mustCreatePR(t, h, map[string]any{"labels": "bug", "assignees": "hubot"})
	pr := onlyPR(t, f)
	if got := labelNames(pr); !sameStrings(got, []string{"bug"}) {
		t.Fatalf("stored labels %v, want [bug]", got)
	}
	if got := assigneeNames(pr); !sameStrings(got, []string{"hubot"}) {
		t.Fatalf("stored assignees %v, want [hubot]", got)
	}
}

func TestCreatePullRequestUnknownLabelFailsAndCreatesNothing(t *testing.T) {
	t.Parallel()
	cases := map[string]func(f *fake.Forgejo) string{
		"unknown name": func(f *fake.Forgejo) string {
			f.SeedIssueLabel(testOwner, testRepo, "bug")
			return "bug,nope"
		},
		"stale id": func(f *fake.Forgejo) string {
			return "999"
		},
	}
	for name, labels := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h, f := newFixture(t)
			res, err := createPR(h, map[string]any{"labels": labels(f)})
			if err == nil && (res == nil || !res.IsError) {
				t.Fatalf("create succeeded, want a failure naming the label: %+v", res)
			}
			if got := listAll(t, f); len(got) != 0 {
				t.Fatalf("a failed create left %d pull request(s) behind", len(got))
			}
		})
	}
}

func TestCreatePullRequestWithoutLabelsOrAssigneesStoresNone(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	mustCreatePR(t, h, nil)
	pr := onlyPR(t, f)
	if len(pr.Labels) != 0 || len(pr.Assignees) != 0 {
		t.Fatalf("stored labels %v and assignees %v, want none", labelNames(pr), assigneeNames(pr))
	}
}
