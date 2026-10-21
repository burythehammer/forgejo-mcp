// SPDX-License-Identifier: GPL-3.0-or-later

package forgetest

import (
	"fmt"
	"testing"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// RunBranchProtections runs the branch-protection contract against the
// implementation that newHarness builds, once per case.
func RunBranchProtections(t *testing.T, newHarness func(t *testing.T) Harness) {
	cases := []struct {
		name string
		run  func(t *testing.T, h Harness)
	}{
		{"unknown repository and rule are not found", func(t *testing.T, h Harness) {
			_, err := h.Port.ListBranchProtections(ctx, "nobody", "nothing", forgejo_sdk.ListBranchProtectionsOptions{})
			wantNotFound(t, err)
			h.Seed.AddRepo(Owner, Repo)
			_, err = h.Port.GetBranchProtection(ctx, Owner, Repo, "main")
			wantNotFound(t, err)
			wantNotFound(t, h.Port.DeleteBranchProtection(ctx, Owner, Repo, "main"))
			_, err = h.Port.EditBranchProtection(ctx, Owner, Repo, "main", forgejo_sdk.EditBranchProtectionOption{})
			wantNotFound(t, err)
		}},
		{"created rule reads back", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			_, err := h.Port.CreateBranchProtection(ctx, Owner, Repo, forgejo_sdk.CreateBranchProtectionOption{
				BranchName: "main", EnableStatusCheck: true, StatusCheckContexts: []string{"ci/build"}, RequiredApprovals: 2,
			})
			must(t, err)
			got, err := h.Port.GetBranchProtection(ctx, Owner, Repo, "main")
			must(t, err)
			if got.RuleName != "main" || got.BranchName != "main" || !got.EnableStatusCheck || got.RequiredApprovals != 2 ||
				len(got.StatusCheckContexts) != 1 || got.StatusCheckContexts[0] != "ci/build" {
				t.Fatalf("got %+v", got)
			}
		}},
		{"a rule that already exists is refused", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			opt := forgejo_sdk.CreateBranchProtectionOption{BranchName: "main"}
			_, err := h.Port.CreateBranchProtection(ctx, Owner, Repo, opt)
			must(t, err)
			if _, err := h.Port.CreateBranchProtection(ctx, Owner, Repo, opt); err == nil {
				t.Fatal("want an error for a duplicate rule")
			}
		}},
		{"edit changes only the fields it sets", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			_, err := h.Port.CreateBranchProtection(ctx, Owner, Repo, forgejo_sdk.CreateBranchProtectionOption{
				BranchName: "main", EnableStatusCheck: true, RequiredApprovals: 1,
			})
			must(t, err)
			on := true
			got, err := h.Port.EditBranchProtection(ctx, Owner, Repo, "main", forgejo_sdk.EditBranchProtectionOption{RequireSignedCommits: &on})
			must(t, err)
			if !got.RequireSignedCommits || !got.EnableStatusCheck || got.RequiredApprovals != 1 {
				t.Fatalf("after edit: %+v", got)
			}
			again, err := h.Port.GetBranchProtection(ctx, Owner, Repo, "main")
			must(t, err)
			if !again.RequireSignedCommits || !again.EnableStatusCheck {
				t.Fatalf("edit did not persist: %+v", again)
			}
		}},
		{"a deleted rule is gone", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			_, err := h.Port.CreateBranchProtection(ctx, Owner, Repo, forgejo_sdk.CreateBranchProtectionOption{BranchName: "main"})
			must(t, err)
			must(t, h.Port.DeleteBranchProtection(ctx, Owner, Repo, "main"))
			_, err = h.Port.GetBranchProtection(ctx, Owner, Repo, "main")
			wantNotFound(t, err)
		}},
		{"list paginates in creation order", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			for i := 1; i <= 3; i++ {
				_, err := h.Port.CreateBranchProtection(ctx, Owner, Repo, forgejo_sdk.CreateBranchProtectionOption{BranchName: fmt.Sprintf("b%d", i)})
				must(t, err)
			}
			p1, err := h.Port.ListBranchProtections(ctx, Owner, Repo, forgejo_sdk.ListBranchProtectionsOptions{ListOptions: forgejo_sdk.ListOptions{Page: 1, PageSize: 2}})
			must(t, err)
			p2, err := h.Port.ListBranchProtections(ctx, Owner, Repo, forgejo_sdk.ListBranchProtectionsOptions{ListOptions: forgejo_sdk.ListOptions{Page: 2, PageSize: 2}})
			must(t, err)
			if len(p1) != 2 || len(p2) != 1 || p1[0].RuleName != "b1" || p1[1].RuleName != "b2" || p2[0].RuleName != "b3" {
				t.Fatalf("page1=%v page2=%v", ruleNames(p1), ruleNames(p2))
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

func ruleNames(bps []*forgejo_sdk.BranchProtection) []string {
	out := make([]string, len(bps))
	for i, b := range bps {
		out[i] = b.RuleName
	}
	return out
}
