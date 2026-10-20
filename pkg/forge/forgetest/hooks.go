// SPDX-License-Identifier: GPL-3.0-or-later

package forgetest

import (
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// RunHooks runs the repository-webhook contract against the implementation
// that newHarness builds, once per case.
func RunHooks(t *testing.T, newHarness func(t *testing.T) Harness) {
	newHookWithFilter := func(t *testing.T, h Harness, url, branchFilter string) *forgejo_sdk.Hook {
		t.Helper()
		h.Seed.AddRepo(Owner, Repo)
		hook, err := h.Port.CreateRepoHook(ctx, Owner, Repo, forgejo_sdk.CreateHookOption{
			Type: forgejo_sdk.HookTypeForgejo, Config: map[string]string{"url": url, "content_type": "json"},
			Events: []string{"push"}, Active: true, BranchFilter: branchFilter,
		})
		must(t, err)
		return hook
	}
	readHook := func(t *testing.T, h Harness, id int64) *forgejo_sdk.Hook {
		t.Helper()
		got, err := h.Port.GetRepoHook(ctx, Owner, Repo, id)
		must(t, err)
		return got
	}
	newHook := func(t *testing.T, h Harness, url string) *forgejo_sdk.Hook {
		t.Helper()
		return newHookWithFilter(t, h, url, "")
	}
	cases := []struct {
		name string
		run  func(t *testing.T, h Harness)
	}{
		{"unknown repository and hook are not found", func(t *testing.T, h Harness) {
			_, err := h.Port.ListRepoHooks(ctx, "nobody", "nothing", forgejo_sdk.ListHooksOptions{})
			wantNotFound(t, err)
			h.Seed.AddRepo(Owner, Repo)
			_, err = h.Port.GetRepoHook(ctx, Owner, Repo, 99)
			wantNotFound(t, err)
			wantNotFound(t, h.Port.DeleteRepoHook(ctx, Owner, Repo, 99))
			wantNotFound(t, h.Port.EditRepoHook(ctx, Owner, Repo, 99, forge.HookEdit{}))
		}},
		{"created hook reads back", func(t *testing.T, h Harness) {
			created := newHook(t, h, "https://example.test/a")
			got, err := h.Port.GetRepoHook(ctx, Owner, Repo, created.ID)
			must(t, err)
			if got.ID != created.ID || got.Type != "forgejo" || !got.Active || got.Config["url"] != "https://example.test/a" ||
				len(got.Events) != 1 || got.Events[0] != "push" {
				t.Fatalf("got %+v", got)
			}
		}},
		{"create without a type is rejected", func(t *testing.T, h Harness) {
			h.Seed.AddRepo(Owner, Repo)
			_, err := h.Port.CreateRepoHook(ctx, Owner, Repo, forgejo_sdk.CreateHookOption{Config: map[string]string{"url": "u"}})
			if err == nil {
				t.Fatal("want an error")
			}
		}},
		{"edit changes the fields it sets", func(t *testing.T, h Harness) {
			created := newHook(t, h, "https://example.test/a")
			off := false
			must(t, h.Port.EditRepoHook(ctx, Owner, Repo, created.ID, forge.HookEdit{
				Config: map[string]string{"url": "https://example.test/b", "content_type": "json"},
				Active: &off,
			}))
			got, err := h.Port.GetRepoHook(ctx, Owner, Repo, created.ID)
			must(t, err)
			if got.Active || got.Config["url"] != "https://example.test/b" || len(got.Events) != 1 {
				t.Fatalf("after edit: %+v", got)
			}
		}},
		{"editing only the config leaves events, active and branch filter unchanged", func(t *testing.T, h Harness) {
			created := newHookWithFilter(t, h, "https://example.test/a", "main")
			must(t, h.Port.EditRepoHook(ctx, Owner, Repo, created.ID, forge.HookEdit{
				Config: map[string]string{"url": "https://example.test/b", "content_type": "json", "branch_filter": "main"},
			}))
			got := readHook(t, h, created.ID)
			if got.Config["url"] != "https://example.test/b" || got.Config["branch_filter"] != "main" ||
				!got.Active || !sameEvents(got.Events, "push") {
				t.Fatalf("after config-only edit: %+v", got)
			}
		}},
		{"editing only the events leaves config, active and branch filter unchanged", func(t *testing.T, h Harness) {
			created := newHookWithFilter(t, h, "https://example.test/a", "main")
			must(t, h.Port.EditRepoHook(ctx, Owner, Repo, created.ID, forge.HookEdit{
				Events: []string{"pull_request"},
			}))
			got := readHook(t, h, created.ID)
			if !sameEvents(got.Events, "pull_request") || got.Config["url"] != "https://example.test/a" ||
				got.Config["branch_filter"] != "main" || !got.Active {
				t.Fatalf("after events-only edit: %+v", got)
			}
		}},
		{"editing only the branch filter leaves config, events and active unchanged", func(t *testing.T, h Harness) {
			created := newHookWithFilter(t, h, "https://example.test/a", "main")
			release := "release/*"
			must(t, h.Port.EditRepoHook(ctx, Owner, Repo, created.ID, forge.HookEdit{BranchFilter: &release}))
			got := readHook(t, h, created.ID)
			if got.Config["branch_filter"] != "release/*" || got.Config["url"] != "https://example.test/a" ||
				!sameEvents(got.Events, "push") || !got.Active {
				t.Fatalf("after branch-filter-only edit: %+v", got)
			}
		}},
		{"editing only active leaves config, events and branch filter unchanged", func(t *testing.T, h Harness) {
			created := newHookWithFilter(t, h, "https://example.test/a", "main")
			off := false
			must(t, h.Port.EditRepoHook(ctx, Owner, Repo, created.ID, forge.HookEdit{Active: &off}))
			got := readHook(t, h, created.ID)
			if got.Active || got.Config["url"] != "https://example.test/a" || got.Config["branch_filter"] != "main" ||
				!sameEvents(got.Events, "push") {
				t.Fatalf("after active-only edit: %+v", got)
			}
		}},
		{"an explicit empty branch filter clears it", func(t *testing.T, h Harness) {
			created := newHookWithFilter(t, h, "https://example.test/a", "main")
			empty := ""
			must(t, h.Port.EditRepoHook(ctx, Owner, Repo, created.ID, forge.HookEdit{BranchFilter: &empty}))
			got := readHook(t, h, created.ID)
			if v, ok := got.Config["branch_filter"]; ok && v != "" {
				t.Fatalf("branch filter = %q after clearing, want empty", v)
			}
			if got.Config["url"] != "https://example.test/a" || !sameEvents(got.Events, "push") || !got.Active {
				t.Fatalf("clearing the branch filter changed other fields: %+v", got)
			}
		}},
		{"a read shows the branch filter", func(t *testing.T, h Harness) {
			created := newHookWithFilter(t, h, "https://example.test/a", "main")
			if got := readHook(t, h, created.ID); got.Config["branch_filter"] != "main" {
				t.Fatalf("get: branch filter = %q, want main", got.Config["branch_filter"])
			}
			h.Seed.AddRepo(Owner, Repo)
			listed, err := h.Port.ListRepoHooks(ctx, Owner, Repo, forgejo_sdk.ListHooksOptions{})
			must(t, err)
			if len(listed.Hooks) != 1 || listed.Hooks[0].Config["branch_filter"] != "main" {
				t.Fatalf("list: %+v, want the branch filter main", listed.Hooks)
			}
		}},
		{"an edit that sets only the config keeps the branch filter", func(t *testing.T, h Harness) {
			created := newHookWithFilter(t, h, "https://example.test/a", "main")
			must(t, h.Port.EditRepoHook(ctx, Owner, Repo, created.ID, forge.HookEdit{
				Config: map[string]string{"url": "https://example.test/b", "content_type": "json"},
			}))
			got := readHook(t, h, created.ID)
			if got.Config["url"] != "https://example.test/b" || got.Config["branch_filter"] != "main" {
				t.Fatalf("after url-only edit: %+v, want the branch filter main kept", got)
			}
		}},
		{"a deleted hook is gone", func(t *testing.T, h Harness) {
			created := newHook(t, h, "https://example.test/a")
			must(t, h.Port.DeleteRepoHook(ctx, Owner, Repo, created.ID))
			_, err := h.Port.GetRepoHook(ctx, Owner, Repo, created.ID)
			wantNotFound(t, err)
		}},
		{"list paginates and reports the total", func(t *testing.T, h Harness) {
			a := newHook(t, h, "https://example.test/a")
			b := newHook(t, h, "https://example.test/b")
			c := newHook(t, h, "https://example.test/c")
			p1, err := h.Port.ListRepoHooks(ctx, Owner, Repo, forgejo_sdk.ListHooksOptions{ListOptions: forgejo_sdk.ListOptions{Page: 1, PageSize: 2}})
			must(t, err)
			p2, err := h.Port.ListRepoHooks(ctx, Owner, Repo, forgejo_sdk.ListHooksOptions{ListOptions: forgejo_sdk.ListOptions{Page: 2, PageSize: 2}})
			must(t, err)
			if len(p1.Hooks) != 2 || len(p2.Hooks) != 1 || p1.Hooks[0].ID != a.ID || p1.Hooks[1].ID != b.ID || p2.Hooks[0].ID != c.ID {
				t.Fatalf("page1=%v page2=%v", p1.Hooks, p2.Hooks)
			}
			if p1.TotalCount == nil || *p1.TotalCount != 3 {
				t.Fatalf("total count = %v, want 3", p1.TotalCount)
			}
		}},
		{"a test delivery of a known hook succeeds, of an unknown one is not found", func(t *testing.T, h Harness) {
			created := newHook(t, h, "https://example.test/a")
			tctx := ctx
			must(t, h.Port.TestRepoHook(tctx, Owner, Repo, created.ID))
			wantNotFound(t, h.Port.TestRepoHook(tctx, Owner, Repo, 99))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.run(t, newHarness(t))
		})
	}
}

// sameEvents reports whether got is exactly the events in want, in order.
func sameEvents(got []string, want ...string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
