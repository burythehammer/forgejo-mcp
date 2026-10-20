// SPDX-License-Identifier: GPL-3.0-or-later

package forgetest

import (
	"fmt"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge/fake"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// RunUsers runs the user contract (profiles and notifications) against the
// implementation that newHarness builds. newHarness is called once per case,
// so cases never share state and run in parallel.
func RunUsers(t *testing.T, newHarness func(t *testing.T) Harness) {
	cases := []struct {
		name string
		run  func(t *testing.T, h Harness)
	}{
		{"my user info is the credential's user", func(t *testing.T, h Harness) {
			me, err := h.Port.GetMyUserInfo(ctx)
			must(t, err)
			if me.UserName != fake.Poster {
				t.Fatalf("got %+v", me)
			}
		}},
		{"a seeded user reads back and an unknown one is not found", func(t *testing.T, h Harness) {
			h.Seed.AddUser("bob")
			got, err := h.Port.GetUserInfo(ctx, "bob")
			must(t, err)
			if got.UserName != "bob" {
				t.Fatalf("got %+v", got)
			}
			_, err = h.Port.GetUserInfo(ctx, "nobody")
			wantNotFound(t, err)
		}},
		{"notifications default to unread, newest first", func(t *testing.T, h Harness) {
			a := h.Seed.AddNotification(Owner, Repo, "a", true)
			h.Seed.AddNotification(Owner, Repo, "seen", false)
			c := h.Seed.AddNotification(Owner, Repo, "c", true)

			got, err := h.Port.ListNotifications(ctx, forgejo_sdk.ListNotificationOptions{})
			must(t, err)
			wantThreads(t, got, c.ID, a.ID)
		}},
		{"notifications filter by status", func(t *testing.T, h Harness) {
			a := h.Seed.AddNotification(Owner, Repo, "a", true)
			b := h.Seed.AddNotification(Owner, Repo, "seen", false)

			read, err := h.Port.ListNotifications(ctx, forgejo_sdk.ListNotificationOptions{Status: []forgejo_sdk.NotifyStatus{forgejo_sdk.NotifyStatusRead}})
			must(t, err)
			wantThreads(t, read, b.ID)
			both, err := h.Port.ListNotifications(ctx, forgejo_sdk.ListNotificationOptions{
				Status: []forgejo_sdk.NotifyStatus{forgejo_sdk.NotifyStatusUnread, forgejo_sdk.NotifyStatusRead},
			})
			must(t, err)
			wantThreads(t, both, b.ID, a.ID)
		}},
		{"notifications paginate", func(t *testing.T, h Harness) {
			var ids []int64
			for i := 0; i < 3; i++ {
				ids = append([]int64{h.Seed.AddNotification(Owner, Repo, fmt.Sprint("n", i), true).ID}, ids...)
			}
			p2, err := h.Port.ListNotifications(ctx, forgejo_sdk.ListNotificationOptions{ListOptions: forgejo_sdk.ListOptions{Page: 2, PageSize: 2}})
			must(t, err)
			wantThreads(t, p2, ids[2])
		}},
		{"repository notifications are scoped to the repository", func(t *testing.T, h Harness) {
			here := h.Seed.AddNotification(Owner, Repo, "here", true)
			h.Seed.AddNotification(Owner, "elsewhere", "there", true)

			got, err := h.Port.ListRepoNotifications(ctx, Owner, Repo, forgejo_sdk.ListNotificationOptions{})
			must(t, err)
			wantThreads(t, got, here.ID)
		}},
		{"a notification thread reads back and an unknown one is not found", func(t *testing.T, h Harness) {
			n := h.Seed.AddNotification(Owner, Repo, "hello", true)
			got, err := h.Port.GetNotification(ctx, n.ID)
			must(t, err)
			if got.ID != n.ID || got.Subject == nil || got.Subject.Title != "hello" || !got.Unread {
				t.Fatalf("got %+v", got)
			}
			_, err = h.Port.GetNotification(ctx, 9999)
			wantNotFound(t, err)
		}},
		{"reading a notification marks it read", func(t *testing.T, h Harness) {
			a := h.Seed.AddNotification(Owner, Repo, "a", true)
			b := h.Seed.AddNotification(Owner, Repo, "b", true)
			got, err := h.Port.ReadNotification(ctx, a.ID)
			must(t, err)
			if got.ID != a.ID {
				t.Fatalf("got %+v", got)
			}
			left, err := h.Port.ListNotifications(ctx, forgejo_sdk.ListNotificationOptions{})
			must(t, err)
			wantThreads(t, left, b.ID)
			_, err = h.Port.ReadNotification(ctx, 9999)
			wantNotFound(t, err)
		}},
		{"reading all notifications marks every thread read", func(t *testing.T, h Harness) {
			h.Seed.AddNotification(Owner, Repo, "a", true)
			h.Seed.AddNotification(Owner, "elsewhere", "b", true)
			must(t, h.Port.ReadNotifications(ctx, forgejo_sdk.MarkNotificationOptions{}))
			left, err := h.Port.ListNotifications(ctx, forgejo_sdk.ListNotificationOptions{})
			must(t, err)
			wantThreads(t, left)
		}},
		{"reading repository notifications leaves other repositories alone", func(t *testing.T, h Harness) {
			h.Seed.AddNotification(Owner, Repo, "here", true)
			there := h.Seed.AddNotification(Owner, "elsewhere", "there", true)
			must(t, h.Port.ReadRepoNotifications(ctx, Owner, Repo, forgejo_sdk.MarkNotificationOptions{}))
			left, err := h.Port.ListNotifications(ctx, forgejo_sdk.ListNotificationOptions{})
			must(t, err)
			wantThreads(t, left, there.ID)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.run(t, newHarness(t))
		})
	}
}

func wantThreads(t *testing.T, got []*forgejo_sdk.NotificationThread, want ...int64) {
	t.Helper()
	have := make([]int64, len(got))
	for i, n := range got {
		have[i] = n.ID
	}
	if fmt.Sprint(have) != fmt.Sprint(append([]int64{}, want...)) {
		t.Fatalf("got thread ids %v, want %v", have, want)
	}
}
