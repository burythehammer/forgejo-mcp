// SPDX-License-Identifier: GPL-3.0-or-later

package forgetest

import (
	"fmt"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

func adminStr(s string) *string { return &s }
func adminBool(b bool) *bool    { return &b }

// RunRepoAdmin runs the repository-administration contract (settings, topics,
// branches) against the implementation newHarness builds.
func RunRepoAdmin(t *testing.T, newHarness func(t *testing.T) Harness) {
	cases := []struct {
		name string
		run  func(t *testing.T, h Harness)
	}{
		{"unknown repository is not found everywhere", func(t *testing.T, h Harness) {
			_, err := h.Port.GetRepo(ctx, "nobody", "nothing")
			wantNotFound(t, err)
			_, err = h.Port.EditRepo(ctx, "nobody", "nothing", forgejo_sdk.EditRepoOption{Description: adminStr("x")})
			wantNotFound(t, err)
			_, err = h.Port.ListRepoTopics(ctx, "nobody", "nothing", forgejo_sdk.ListRepoTopicsOptions{})
			wantNotFound(t, err)
			wantNotFound(t, h.Port.SetRepoTopics(ctx, "nobody", "nothing", []string{"a"}))
			wantNotFound(t, h.Port.AddRepoTopic(ctx, "nobody", "nothing", "a"))
			wantNotFound(t, h.Port.DeleteRepoTopic(ctx, "nobody", "nothing", "a"))
			_, err = h.Port.ListBranches(ctx, "nobody", "nothing", forgejo_sdk.ListRepoBranchesOptions{})
			wantNotFound(t, err)
			_, err = h.Port.CreateBranch(ctx, "nobody", "nothing", forgejo_sdk.CreateBranchOption{BranchName: "b"})
			wantNotFound(t, err)
			wantRefusedDelete(t, h, "nobody", "nothing", "b", 404, "repository not found")
		}},
		{"created repository reads back and is listed as mine", func(t *testing.T, h Harness) {
			r, err := h.Port.CreateRepo(ctx, forgejo_sdk.CreateRepoOption{Name: "fresh", Description: "d", Private: true})
			must(t, err)
			if r.Name != "fresh" || r.Description != "d" || !r.Private {
				t.Fatalf("created %+v", r)
			}
			got, err := h.Port.GetRepo(ctx, r.Owner.UserName, "fresh")
			must(t, err)
			if got.Name != "fresh" {
				t.Fatalf("read back %+v", got)
			}
			mine, err := h.Port.ListMyRepos(ctx, forgejo_sdk.ListReposOptions{})
			must(t, err)
			if len(mine) != 1 || mine[0].Name != "fresh" {
				t.Fatalf("my repos: %+v", mine)
			}
			_, err = h.Port.CreateRepo(ctx, forgejo_sdk.CreateRepoOption{Name: "fresh"})
			if err == nil {
				t.Fatal("duplicate repository must be refused")
			}
		}},
		{"creating in an unknown organization is not found", func(t *testing.T, h Harness) {
			_, err := h.Port.CreateOrgRepo(ctx, "no-such-org", forgejo_sdk.CreateRepoOption{Name: "x"})
			wantNotFound(t, err)
		}},
		{"fork copies the source under the caller; unknown source is not found", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			f, err := h.Port.ForkRepo(ctx, Owner, Repo, forgejo_sdk.CreateForkOption{})
			must(t, err)
			if f.Name != Repo || f.Owner.UserName == Owner {
				t.Fatalf("fork %+v", f)
			}
			_, err = h.Port.GetRepo(ctx, f.Owner.UserName, Repo)
			must(t, err)
			_, err = h.Port.ForkRepo(ctx, "nobody", "nothing", forgejo_sdk.CreateForkOption{})
			wantNotFound(t, err)
		}},
		{"get repo returns its identity", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			r, err := h.Port.GetRepo(ctx, Owner, Repo)
			must(t, err)
			if r.Name != Repo || r.FullName != Owner+"/"+Repo || r.DefaultBranch != "main" {
				t.Fatalf("got name=%q full=%q default=%q", r.Name, r.FullName, r.DefaultBranch)
			}
		}},
		{"edit changes only the supplied fields", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			_, err := h.Port.EditRepo(ctx, Owner, Repo, forgejo_sdk.EditRepoOption{Description: adminStr("about"), Website: adminStr("https://x.example")})
			must(t, err)
			r, err := h.Port.EditRepo(ctx, Owner, Repo, forgejo_sdk.EditRepoOption{Private: adminBool(true), Archived: adminBool(true)})
			must(t, err)
			if r.Description != "about" || r.Website != "https://x.example" || !r.Private || !r.Archived {
				t.Fatalf("after edit: %+v", r)
			}
			got, err := h.Port.GetRepo(ctx, Owner, Repo)
			must(t, err)
			if got.Description != "about" || !got.Private || !got.Archived {
				t.Fatalf("read back: %+v", got)
			}
			// An empty string clears; false switches a unit off.
			r, err = h.Port.EditRepo(ctx, Owner, Repo, forgejo_sdk.EditRepoOption{Description: adminStr(""), Private: adminBool(false), HasIssues: adminBool(false)})
			must(t, err)
			if r.Description != "" || r.Private || r.HasIssues || !r.Archived {
				t.Fatalf("after clearing: %+v", r)
			}
		}},
		{"edit to a default branch that does not exist is rejected", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			_, err := h.Port.EditRepo(ctx, Owner, Repo, forgejo_sdk.EditRepoOption{DefaultBranch: adminStr("nope")})
			if err == nil {
				t.Fatal("want an error")
			}
			_, err = h.Port.CreateBranch(ctx, Owner, Repo, forgejo_sdk.CreateBranchOption{BranchName: "trunk", OldBranchName: "main"})
			must(t, err)
			r, err := h.Port.EditRepo(ctx, Owner, Repo, forgejo_sdk.EditRepoOption{DefaultBranch: adminStr("trunk")})
			must(t, err)
			if r.DefaultBranch != "trunk" {
				t.Fatalf("default branch %q", r.DefaultBranch)
			}
		}},
		{"rename moves the repository", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			r, err := h.Port.EditRepo(ctx, Owner, Repo, forgejo_sdk.EditRepoOption{Name: adminStr("renamed")})
			must(t, err)
			if r.Name != "renamed" {
				t.Fatalf("name %q", r.Name)
			}
			_, err = h.Port.GetRepo(ctx, Owner, "renamed")
			must(t, err)
			_, err = h.Port.GetRepo(ctx, Owner, Repo)
			wantNotFound(t, err)
		}},
		{"topics set, add, delete and read back", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			got, err := h.Port.ListRepoTopics(ctx, Owner, Repo, forgejo_sdk.ListRepoTopicsOptions{})
			must(t, err)
			if len(got) != 0 {
				t.Fatalf("new repository has topics %v", got)
			}
			must(t, h.Port.SetRepoTopics(ctx, Owner, Repo, []string{"go", "mcp"}))
			must(t, h.Port.AddRepoTopic(ctx, Owner, Repo, "forge"))
			got, err = h.Port.ListRepoTopics(ctx, Owner, Repo, forgejo_sdk.ListRepoTopicsOptions{})
			must(t, err)
			if fmt.Sprint(got) != "[go mcp forge]" {
				t.Fatalf("got %v", got)
			}
			must(t, h.Port.DeleteRepoTopic(ctx, Owner, Repo, "mcp"))
			got, err = h.Port.ListRepoTopics(ctx, Owner, Repo, forgejo_sdk.ListRepoTopicsOptions{})
			must(t, err)
			if fmt.Sprint(got) != "[go forge]" {
				t.Fatalf("got %v", got)
			}
			must(t, h.Port.SetRepoTopics(ctx, Owner, Repo, []string{}))
			got, err = h.Port.ListRepoTopics(ctx, Owner, Repo, forgejo_sdk.ListRepoTopicsOptions{})
			must(t, err)
			if len(got) != 0 {
				t.Fatalf("after clear: %v", got)
			}
		}},
		{"deleting an absent topic is not found; adding twice is harmless", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			wantNotFound(t, h.Port.DeleteRepoTopic(ctx, Owner, Repo, "ghost"))
			must(t, h.Port.AddRepoTopic(ctx, Owner, Repo, "twice"))
			must(t, h.Port.AddRepoTopic(ctx, Owner, Repo, "twice"))
			got, err := h.Port.ListRepoTopics(ctx, Owner, Repo, forgejo_sdk.ListRepoTopicsOptions{})
			must(t, err)
			if fmt.Sprint(got) != "[twice]" {
				t.Fatalf("got %v", got)
			}
		}},
		{"topics paginate", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			must(t, h.Port.SetRepoTopics(ctx, Owner, Repo, []string{"a", "b", "c"}))
			p1, err := h.Port.ListRepoTopics(ctx, Owner, Repo, forgejo_sdk.ListRepoTopicsOptions{ListOptions: forgejo_sdk.ListOptions{Page: 1, PageSize: 2}})
			must(t, err)
			p2, err := h.Port.ListRepoTopics(ctx, Owner, Repo, forgejo_sdk.ListRepoTopicsOptions{ListOptions: forgejo_sdk.ListOptions{Page: 2, PageSize: 2}})
			must(t, err)
			if fmt.Sprint(p1) != "[a b]" || fmt.Sprint(p2) != "[c]" {
				t.Fatalf("got %v then %v", p1, p2)
			}
		}},
		{"a new repository has its default branch", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			bs, err := h.Port.ListBranches(ctx, Owner, Repo, forgejo_sdk.ListRepoBranchesOptions{})
			must(t, err)
			if fmt.Sprint(adminBranchNames(bs)) != "[main]" {
				t.Fatalf("got %v", adminBranchNames(bs))
			}
		}},
		{"created branch reads back and paginates", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			b, err := h.Port.CreateBranch(ctx, Owner, Repo, forgejo_sdk.CreateBranchOption{BranchName: "feat-a", OldBranchName: "main"})
			must(t, err)
			if b.Name != "feat-a" {
				t.Fatalf("name %q", b.Name)
			}
			_, err = h.Port.CreateBranch(ctx, Owner, Repo, forgejo_sdk.CreateBranchOption{BranchName: "feat-b"})
			must(t, err)
			all, err := h.Port.ListBranches(ctx, Owner, Repo, forgejo_sdk.ListRepoBranchesOptions{})
			must(t, err)
			if len(all) != 3 {
				t.Fatalf("got %v", adminBranchNames(all))
			}
			p2, err := h.Port.ListBranches(ctx, Owner, Repo, forgejo_sdk.ListRepoBranchesOptions{ListOptions: forgejo_sdk.ListOptions{Page: 2, PageSize: 2}})
			must(t, err)
			if len(p2) != 1 {
				t.Fatalf("page 2 got %v", adminBranchNames(p2))
			}
		}},
		{"creating a branch from an unknown branch is not found, a duplicate is refused", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			_, err := h.Port.CreateBranch(ctx, Owner, Repo, forgejo_sdk.CreateBranchOption{BranchName: "x", OldBranchName: "ghost"})
			wantNotFound(t, err)
			_, err = h.Port.CreateBranch(ctx, Owner, Repo, forgejo_sdk.CreateBranchOption{BranchName: "main", OldBranchName: "main"})
			if err == nil {
				t.Fatal("duplicate branch must be refused")
			}
		}},
		{"deleted branch is gone; unknown and default are refused", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			_, err := h.Port.CreateBranch(ctx, Owner, Repo, forgejo_sdk.CreateBranchOption{BranchName: "tmp", OldBranchName: "main"})
			must(t, err)
			ok, err := h.Port.DeleteBranch(ctx, Owner, Repo, "tmp")
			must(t, err)
			if !ok {
				t.Fatal("delete not reported")
			}
			bs, err := h.Port.ListBranches(ctx, Owner, Repo, forgejo_sdk.ListRepoBranchesOptions{})
			must(t, err)
			if fmt.Sprint(adminBranchNames(bs)) != "[main]" {
				t.Fatalf("got %v", adminBranchNames(bs))
			}
			wantRefusedDelete(t, h, Owner, Repo, "tmp", 404, "branch does not exist")
			wantRefusedDelete(t, h, Owner, Repo, "main", 403, "default branch")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.run(t, newHarness(t))
		})
	}
}

// wantRefusedDelete: Forgejo refuses a branch delete (unknown repository or
// branch, the default branch) with a status and a message. The port reports it
// as an error that keeps both, and nothing is reported deleted.
func wantRefusedDelete(t *testing.T, h Harness, owner, repo, branch string, status int, message string) {
	t.Helper()
	ok, err := h.Port.DeleteBranch(ctx, owner, repo, branch)
	if ok || err == nil {
		t.Fatalf("delete of %s/%s %q: ok=%v err=%v, want a refusal", owner, repo, branch, ok, err)
	}
	if forge.StatusCode(err) != status || !strings.Contains(err.Error(), message) {
		t.Fatalf("delete of %s/%s %q: got %v (status %d), want status %d with %q", owner, repo, branch, err, forge.StatusCode(err), status, message)
	}
}

func adminBranchNames(bs []*forgejo_sdk.Branch) []string {
	out := make([]string, len(bs))
	for i, b := range bs {
		out[i] = b.Name
	}
	return out
}
