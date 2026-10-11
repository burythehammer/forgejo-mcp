// SPDX-License-Identifier: GPL-3.0-or-later

package forgetest

import (
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// RunHooks runs the repository-webhook contract against the implementation
// that newHarness builds, once per case.
func RunHooks(t *testing.T, newHarness func(t *testing.T) Harness) {
	newHook := func(t *testing.T, h Harness, url string) *forgejo_sdk.Hook {
		t.Helper()
		h.Seed.AddRepo(Owner, Repo)
		hook, err := h.Port.CreateRepoHook(ctx, Owner, Repo, forgejo_sdk.CreateHookOption{
			Type: forgejo_sdk.HookTypeForgejo, Config: map[string]string{"url": url, "content_type": "json"},
			Events: []string{"push"}, Active: true,
		})
		must(t, err)
		return hook
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
			wantNotFound(t, h.Port.EditRepoHook(ctx, Owner, Repo, 99, forgejo_sdk.EditHookOption{}))
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
			must(t, h.Port.EditRepoHook(ctx, Owner, Repo, created.ID, forgejo_sdk.EditHookOption{
				Config: map[string]string{"url": "https://example.test/b", "content_type": "json"},
				Active: &off,
			}))
			got, err := h.Port.GetRepoHook(ctx, Owner, Repo, created.ID)
			must(t, err)
			if got.Active || got.Config["url"] != "https://example.test/b" || len(got.Events) != 1 {
				t.Fatalf("after edit: %+v", got)
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
			// The raw-HTTP path carries the request's credential in ctx.
			tctx := forgejo.WithToken(ctx, "stub-token")
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
