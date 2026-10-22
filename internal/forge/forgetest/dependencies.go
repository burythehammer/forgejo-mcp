// SPDX-License-Identifier: GPL-3.0-or-later

package forgetest

import (
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// RunDependencies runs the issue-dependency contract against the
// implementation that newHarness builds. newHarness is called once per case.
func RunDependencies(t *testing.T, newHarness func(t *testing.T) Harness) {
	cases := []struct {
		name string
		run  func(t *testing.T, h Harness)
	}{
		{"a dependency reads back from both sides", func(t *testing.T, h Harness) {
			a := newIssue(t, h, "a")
			b := newIssue(t, h, "b")
			must(t, h.Port.AddIssueDependency(ctx, Owner, Repo, a.Index, forge.IssueRef{Owner: Owner, Repo: Repo, Index: b.Index}))
			deps, err := h.Port.ListIssueDependencies(ctx, Owner, Repo, a.Index, 1, 10)
			must(t, err)
			wantIssueIndices(t, deps, b.Index)
			dependents, err := h.Port.ListIssueDependents(ctx, Owner, Repo, b.Index, 1, 10)
			must(t, err)
			wantIssueIndices(t, dependents, a.Index)
			none, err := h.Port.ListIssueDependencies(ctx, Owner, Repo, b.Index, 1, 10)
			must(t, err)
			wantIssueIndices(t, none)
		}},
		{"a dependency may live in another repository", func(t *testing.T, h Harness) {
			a := newIssue(t, h, "a")
			h.Seed.AddRepo("other", "repo")
			b := newIssueIn(t, h, "other", "repo", "b")
			must(t, h.Port.AddIssueDependency(ctx, Owner, Repo, a.Index, forge.IssueRef{Owner: "other", Repo: "repo", Index: b.Index}))
			deps, err := h.Port.ListIssueDependencies(ctx, Owner, Repo, a.Index, 1, 10)
			must(t, err)
			wantIssueIndices(t, deps, b.Index)
			dependents, err := h.Port.ListIssueDependents(ctx, "other", "repo", b.Index, 1, 10)
			must(t, err)
			wantIssueIndices(t, dependents, a.Index)
		}},
		{"removing a dependency", func(t *testing.T, h Harness) {
			a := newIssue(t, h, "a")
			b := newIssue(t, h, "b")
			ref := forge.IssueRef{Owner: Owner, Repo: Repo, Index: b.Index}
			must(t, h.Port.AddIssueDependency(ctx, Owner, Repo, a.Index, ref))
			must(t, h.Port.RemoveIssueDependency(ctx, Owner, Repo, a.Index, ref))
			deps, err := h.Port.ListIssueDependencies(ctx, Owner, Repo, a.Index, 1, 10)
			must(t, err)
			wantIssueIndices(t, deps)
			wantNotFound(t, h.Port.RemoveIssueDependency(ctx, Owner, Repo, a.Index, ref))
		}},
		{"a dependency on an unknown issue is not found", func(t *testing.T, h Harness) {
			a := newIssue(t, h, "a")
			wantNotFound(t, h.Port.AddIssueDependency(ctx, Owner, Repo, a.Index, forge.IssueRef{Owner: Owner, Repo: Repo, Index: 99}))
			wantNotFound(t, h.Port.AddIssueDependency(ctx, Owner, Repo, 99, forge.IssueRef{Owner: Owner, Repo: Repo, Index: a.Index}))
		}},
		{"a duplicate dependency conflicts", func(t *testing.T, h Harness) {
			a := newIssue(t, h, "a")
			b := newIssue(t, h, "b")
			ref := forge.IssueRef{Owner: Owner, Repo: Repo, Index: b.Index}
			must(t, h.Port.AddIssueDependency(ctx, Owner, Repo, a.Index, ref))
			if err := h.Port.AddIssueDependency(ctx, Owner, Repo, a.Index, ref); forge.StatusCode(err) != 409 {
				t.Fatalf("got %v (status %d), want 409", err, forge.StatusCode(err))
			}
		}},
		{"dependency lists paginate", func(t *testing.T, h Harness) {
			a := newIssue(t, h, "a")
			var want []int64
			for _, title := range []string{"b", "c", "d"} {
				dep := newIssue(t, h, title)
				want = append(want, dep.Index)
				must(t, h.Port.AddIssueDependency(ctx, Owner, Repo, a.Index, forge.IssueRef{Owner: Owner, Repo: Repo, Index: dep.Index}))
			}
			first, err := h.Port.ListIssueDependencies(ctx, Owner, Repo, a.Index, 1, 2)
			must(t, err)
			wantIssueIndices(t, first, want[0], want[1])
			second, err := h.Port.ListIssueDependencies(ctx, Owner, Repo, a.Index, 2, 2)
			must(t, err)
			wantIssueIndices(t, second, want[2])
		}},
		{"dependencies and dependents of a missing issue are not found", func(t *testing.T, h Harness) {
			a := newIssue(t, h, "a")
			_, err := h.Port.ListIssueDependencies(ctx, Owner, Repo, 99, 1, 10)
			wantNotFound(t, err)
			_, err = h.Port.ListIssueDependents(ctx, Owner, Repo, 99, 1, 10)
			wantNotFound(t, err)
			// An existing issue with nothing related is an empty list, not not-found.
			deps, err := h.Port.ListIssueDependencies(ctx, Owner, Repo, a.Index, 1, 10)
			must(t, err)
			wantIssueIndices(t, deps)
			dependents, err := h.Port.ListIssueDependents(ctx, Owner, Repo, a.Index, 1, 10)
			must(t, err)
			wantIssueIndices(t, dependents)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.run(t, newHarness(t))
		})
	}
}

// newIssueIn creates an issue in an existing repository.
func newIssueIn(t *testing.T, h Harness, owner, repo, title string) *forgejo_sdk.Issue {
	t.Helper()
	iss, err := h.Port.CreateIssue(ctx, owner, repo, forgejo_sdk.CreateIssueOption{Title: title})
	must(t, err)
	return iss
}
