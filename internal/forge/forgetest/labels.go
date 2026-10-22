// SPDX-License-Identifier: GPL-3.0-or-later

package forgetest

import (
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// forgeNames and sdkNames give the names of a label list: forgeNames for the
// repository and organization labels, sdkNames for the labels an issue carries.
func forgeNames(ls []*forge.Label) []string {
	out := make([]string, len(ls))
	for i, l := range ls {
		out[i] = l.Name
	}
	return out
}

func sdkNames(ls []*forgejo_sdk.Label) []string {
	out := make([]string, len(ls))
	for i, l := range ls {
		out[i] = l.Name
	}
	return out
}

func wantNames(t *testing.T, have []string, want ...string) {
	t.Helper()
	if len(have) != len(want) {
		t.Fatalf("labels = %v, want %v", have, want)
	}
	for i := range want {
		if have[i] != want[i] {
			t.Fatalf("labels = %v, want %v", have, want)
		}
	}
}

// RunLabels runs the label contract against the implementation that
// newHarness builds. newHarness is called once per case.
func RunLabels(t *testing.T, newHarness func(t *testing.T) Harness) {
	str := func(s string) *string { return &s }
	flag := func(b bool) *bool { return &b }
	cases := []struct {
		name string
		run  func(t *testing.T, h Harness)
	}{
		{"repo label round-trips", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			l, err := h.Port.CreateRepoLabel(ctx, Owner, Repo, forge.LabelCreate{Name: "bug", Color: "#ff0000", Description: "broken"})
			must(t, err)
			got, err := h.Port.GetRepoLabel(ctx, Owner, Repo, l.ID)
			must(t, err)
			if got.Name != "bug" || got.Description != "broken" || got.ID != l.ID {
				t.Fatalf("got %+v", got)
			}
			_, err = h.Port.GetRepoLabel(ctx, Owner, Repo, 9999)
			wantNotFound(t, err)
			_, err = h.Port.GetRepoLabel(ctx, "nobody", "nothing", l.ID)
			wantNotFound(t, err)
		}},
		{"repo label edit changes only the supplied fields", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			l, err := h.Port.CreateRepoLabel(ctx, Owner, Repo, forge.LabelCreate{Name: "bug", Color: "#ff0000", Description: "broken"})
			must(t, err)
			got, err := h.Port.EditRepoLabel(ctx, Owner, Repo, l.ID, forge.LabelPatch{Name: str("defect")})
			must(t, err)
			if got.Name != "defect" || got.Description != "broken" {
				t.Fatalf("got %+v", got)
			}
			_, err = h.Port.EditRepoLabel(ctx, Owner, Repo, 9999, forge.LabelPatch{Name: str("x")})
			wantNotFound(t, err)
		}},
		{"exclusive round-trips on repo and org labels", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			on, err := h.Port.CreateRepoLabel(ctx, Owner, Repo, forge.LabelCreate{Name: "status/ready", Color: "#00ff00", Exclusive: true})
			must(t, err)
			if !on.Exclusive {
				t.Fatalf("create: exclusive not stored: %+v", on)
			}
			off, err := h.Port.CreateRepoLabel(ctx, Owner, Repo, forge.LabelCreate{Name: "plain", Color: "#00ff00"})
			must(t, err)
			got, err := h.Port.GetRepoLabel(ctx, Owner, Repo, on.ID)
			must(t, err)
			if !got.Exclusive {
				t.Fatalf("get: exclusive lost: %+v", got)
			}
			// An edit that omits Exclusive keeps the stored value.
			got, err = h.Port.EditRepoLabel(ctx, Owner, Repo, on.ID, forge.LabelPatch{Name: str("status/done")})
			must(t, err)
			if !got.Exclusive {
				t.Fatalf("rename reset exclusive: %+v", got)
			}
			got, err = h.Port.EditRepoLabel(ctx, Owner, Repo, on.ID, forge.LabelPatch{Exclusive: flag(false)})
			must(t, err)
			if got.Exclusive {
				t.Fatalf("edit to false not stored: %+v", got)
			}
			got, err = h.Port.EditRepoLabel(ctx, Owner, Repo, off.ID, forge.LabelPatch{Exclusive: flag(true)})
			must(t, err)
			if !got.Exclusive {
				t.Fatalf("edit to true not stored: %+v", got)
			}
			list, _, err := h.Port.ListRepoLabels(ctx, Owner, Repo, 1, 10)
			must(t, err)
			for _, l := range list {
				if l.ID == off.ID && !l.Exclusive {
					t.Fatalf("list: exclusive not shown: %+v", l)
				}
			}
			org, err := h.Port.CreateOrgLabel(ctx, "acme", forge.LabelCreate{Name: "prio/1", Color: "#0000ff", Exclusive: true})
			must(t, err)
			if !org.Exclusive {
				t.Fatalf("org create: exclusive not stored: %+v", org)
			}
			got2, err := h.Port.GetOrgLabel(ctx, "acme", org.ID)
			must(t, err)
			if !got2.Exclusive {
				t.Fatalf("org get: exclusive lost: %+v", got2)
			}
			got2, err = h.Port.EditOrgLabel(ctx, "acme", org.ID, forge.LabelPatch{Exclusive: flag(false)})
			must(t, err)
			if got2.Exclusive {
				t.Fatalf("org edit to false not stored: %+v", got2)
			}
		}},
		{"repo label delete removes it from issues", func(t *testing.T, h Harness) {
			iss := newIssue(t, h, "one")
			l, err := h.Port.CreateRepoLabel(ctx, Owner, Repo, forge.LabelCreate{Name: "bug", Color: "#ff0000"})
			must(t, err)
			must(t, h.Port.AddIssueLabels(ctx, Owner, Repo, iss.Index, []int64{l.ID}))
			must(t, h.Port.DeleteRepoLabel(ctx, Owner, Repo, l.ID))
			wantNotFound(t, h.Port.DeleteRepoLabel(ctx, Owner, Repo, l.ID))
			wantLabels(t, h, iss.Index)
		}},
		{"repo labels paginate", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			for _, n := range []string{"a", "b", "c"} {
				_, err := h.Port.CreateRepoLabel(ctx, Owner, Repo, forge.LabelCreate{Name: n, Color: "#00ff00"})
				must(t, err)
			}
			first, info, err := h.Port.ListRepoLabels(ctx, Owner, Repo, 1, 2)
			must(t, err)
			wantNames(t, forgeNames(first), "a", "b")
			if !info.HasNext || !info.TotalKnown || info.TotalCount != 3 {
				t.Fatalf("page info %+v", info)
			}
			second, info, err := h.Port.ListRepoLabels(ctx, Owner, Repo, 2, 2)
			must(t, err)
			wantNames(t, forgeNames(second), "c")
			if info.HasNext {
				t.Fatalf("page info %+v", info)
			}
			_, _, err = h.Port.ListRepoLabels(ctx, "nobody", "nothing", 1, 10)
			wantNotFound(t, err)
		}},
		{"org label round-trips and edits", func(t *testing.T, h Harness) {
			l, err := h.Port.CreateOrgLabel(ctx, "acme", forge.LabelCreate{Name: "prio", Color: "#0000ff", Description: "d"})
			must(t, err)
			got, err := h.Port.GetOrgLabel(ctx, "acme", l.ID)
			must(t, err)
			if got.Name != "prio" || got.Description != "d" {
				t.Fatalf("got %+v", got)
			}
			got, err = h.Port.EditOrgLabel(ctx, "acme", l.ID, forge.LabelPatch{Description: str("")})
			must(t, err)
			if got.Name != "prio" || got.Description != "" {
				t.Fatalf("edit: %+v", got)
			}
			_, err = h.Port.GetOrgLabel(ctx, "acme", 9999)
			wantNotFound(t, err)
			must(t, h.Port.DeleteOrgLabel(ctx, "acme", l.ID))
			wantNotFound(t, h.Port.DeleteOrgLabel(ctx, "acme", l.ID))
		}},
		{"org labels paginate", func(t *testing.T, h Harness) {
			_, err := h.Port.CreateOrg(ctx, forgejo_sdk.CreateOrgOption{Name: "acme"})
			must(t, err)
			for _, n := range []string{"a", "b", "c"} {
				_, err := h.Port.CreateOrgLabel(ctx, "acme", forge.LabelCreate{Name: n, Color: "#0000ff"})
				must(t, err)
			}
			first, info, err := h.Port.ListOrgLabels(ctx, "acme", 1, 2)
			must(t, err)
			wantNames(t, forgeNames(first), "a", "b")
			if !info.HasNext || info.TotalCount != 3 {
				t.Fatalf("page info %+v", info)
			}
			second, _, err := h.Port.ListOrgLabels(ctx, "acme", 2, 2)
			must(t, err)
			wantNames(t, forgeNames(second), "c")
		}},
		{"org labels of an existing org with none are empty", func(t *testing.T, h Harness) {
			_, err := h.Port.CreateOrg(ctx, forgejo_sdk.CreateOrgOption{Name: "vacant"})
			must(t, err)
			none, _, err := h.Port.ListOrgLabels(ctx, "vacant", 1, 10)
			must(t, err)
			wantNames(t, forgeNames(none))
		}},
		{"org labels of an unknown org are not found", func(t *testing.T, h Harness) {
			_, _, err := h.Port.ListOrgLabels(ctx, "ghost", 1, 10)
			wantNotFound(t, err)
		}},
		{"issue labels add and remove, org labels included", func(t *testing.T, h Harness) {
			h.Seed.AddRepo("acme", "anvil")
			iss, err := h.Port.CreateIssue(ctx, "acme", "anvil", forgejo_sdk.CreateIssueOption{Title: "t"})
			must(t, err)
			repoL, err := h.Port.CreateRepoLabel(ctx, "acme", "anvil", forge.LabelCreate{Name: "bug", Color: "#ff0000"})
			must(t, err)
			orgL, err := h.Port.CreateOrgLabel(ctx, "acme", forge.LabelCreate{Name: "prio", Color: "#0000ff"})
			must(t, err)
			must(t, h.Port.AddIssueLabels(ctx, "acme", "anvil", iss.Index, []int64{repoL.ID}))
			must(t, h.Port.AddIssueLabels(ctx, "acme", "anvil", iss.Index, []int64{orgL.ID}))
			got, err := h.Port.GetIssue(ctx, "acme", "anvil", iss.Index)
			must(t, err)
			wantNames(t, sdkNames(got.Labels), "bug", "prio")
			must(t, h.Port.RemoveIssueLabel(ctx, "acme", "anvil", iss.Index, repoL.ID))
			got, err = h.Port.GetIssue(ctx, "acme", "anvil", iss.Index)
			must(t, err)
			wantNames(t, sdkNames(got.Labels), "prio")
			wantNotFound(t, h.Port.AddIssueLabels(ctx, "acme", "anvil", iss.Index, []int64{9999}))
			wantNotFound(t, h.Port.AddIssueLabels(ctx, "acme", "anvil", 99, []int64{orgL.ID}))
		}},
		{"org repo names page", func(t *testing.T, h Harness) {
			for _, r := range []string{"anvil", "rope", "saw"} {
				h.Seed.AddRepo("acme", r)
			}
			h.Seed.AddRepo("other", "x")
			first, err := h.Port.ListOrgRepoNames(ctx, "acme", 1, 2)
			must(t, err)
			if len(first) != 2 || first[0] != "anvil" || first[1] != "rope" {
				t.Fatalf("first page %v", first)
			}
			second, err := h.Port.ListOrgRepoNames(ctx, "acme", 2, 2)
			must(t, err)
			if len(second) != 1 || second[0] != "saw" {
				t.Fatalf("second page %v", second)
			}
			end, err := h.Port.ListOrgRepoNames(ctx, "acme", 3, 2)
			must(t, err)
			if len(end) != 0 {
				t.Fatalf("past the end %v", end)
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
