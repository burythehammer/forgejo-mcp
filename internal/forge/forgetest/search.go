// SPDX-License-Identifier: GPL-3.0-or-later

package forgetest

import (
	"fmt"
	"testing"
	"time"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// RunSearch runs the search contract against the implementation that
// newHarness builds, once per case.
func RunSearch(t *testing.T, newHarness func(t *testing.T) Harness) {
	cases := []struct {
		name string
		run  func(t *testing.T, h Harness)
	}{
		{"users match the keyword and paginate", func(t *testing.T, h Harness) {
			for _, n := range []string{"alice", "alicia", "bob"} {
				h.Seed.SeedUser(forgejo_sdk.User{UserName: n})
			}
			got, err := h.Port.SearchUsers(ctx, forgejo_sdk.SearchUsersOption{KeyWord: "ali"})
			must(t, err)
			wantUsers(t, got, "alice", "alicia")
			all, err := h.Port.SearchUsers(ctx, forgejo_sdk.SearchUsersOption{})
			must(t, err)
			wantUsers(t, all, "alice", "alicia", "bob")
			second, err := h.Port.SearchUsers(ctx, forgejo_sdk.SearchUsersOption{ListOptions: forgejo_sdk.ListOptions{Page: 2, PageSize: 2}})
			must(t, err)
			wantUsers(t, second, "bob")
			none, err := h.Port.SearchUsers(ctx, forgejo_sdk.SearchUsersOption{KeyWord: "zzz"})
			must(t, err)
			wantUsers(t, none)
		}},
		{"teams are searched within an organization", func(t *testing.T, h Harness) {
			h.Seed.SeedTeam("acme", forgejo_sdk.Team{Name: "owners", Description: "the bosses"})
			h.Seed.SeedTeam("acme", forgejo_sdk.Team{Name: "devs", Description: "builds things"})
			h.Seed.SeedTeam("other", forgejo_sdk.Team{Name: "devs"})
			got, err := h.Port.SearchOrgTeams(ctx, "acme", forgejo_sdk.SearchTeamsOptions{Query: "dev"})
			must(t, err)
			if len(got) != 1 || got[0].Name != "devs" {
				t.Fatalf("got %+v", got)
			}
			all, err := h.Port.SearchOrgTeams(ctx, "acme", forgejo_sdk.SearchTeamsOptions{})
			must(t, err)
			if len(all) != 2 {
				t.Fatalf("got %+v", all)
			}
			byDesc, err := h.Port.SearchOrgTeams(ctx, "acme", forgejo_sdk.SearchTeamsOptions{Query: "bosses", IncludeDescription: true})
			must(t, err)
			if len(byDesc) != 1 || byDesc[0].Name != "owners" {
				t.Fatalf("got %+v", byDesc)
			}
		}},
		{"teams of an unknown organization are not found", func(t *testing.T, h Harness) {
			_, err := h.Port.SearchOrgTeams(ctx, "ghost", forgejo_sdk.SearchTeamsOptions{})
			wantNotFound(t, err)
		}},
		{"repositories match the keyword and sort", func(t *testing.T, h Harness) {
			base := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
			h.Seed.SeedSearchableRepo(forgejo_sdk.Repository{ID: 1, Name: "tool-b", Updated: base.Add(time.Hour)})
			h.Seed.SeedSearchableRepo(forgejo_sdk.Repository{ID: 2, Name: "tool-a", Updated: base.Add(2 * time.Hour)})
			h.Seed.SeedSearchableRepo(forgejo_sdk.Repository{ID: 3, Name: "other", Updated: base})
			byName, err := h.Port.SearchRepos(ctx, forgejo_sdk.SearchRepoOptions{Keyword: "tool"})
			must(t, err)
			wantRepos(t, byName, "tool-a", "tool-b")
			newest, err := h.Port.SearchRepos(ctx, forgejo_sdk.SearchRepoOptions{Keyword: "tool", Sort: "updated", Order: "desc"})
			must(t, err)
			wantRepos(t, newest, "tool-a", "tool-b")
			oldest, err := h.Port.SearchRepos(ctx, forgejo_sdk.SearchRepoOptions{Sort: "updated", Order: "asc"})
			must(t, err)
			wantRepos(t, oldest, "other", "tool-b", "tool-a")
			second, err := h.Port.SearchRepos(ctx, forgejo_sdk.SearchRepoOptions{ListOptions: forgejo_sdk.ListOptions{Page: 2, PageSize: 2}})
			must(t, err)
			wantRepos(t, second, "tool-b")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.run(t, newHarness(t))
		})
	}
}

func wantUsers(t *testing.T, got []*forgejo_sdk.User, want ...string) {
	t.Helper()
	have := make([]string, len(got))
	for i, u := range got {
		have[i] = u.UserName
	}
	if fmt.Sprint(have) != fmt.Sprint(append([]string{}, want...)) {
		t.Fatalf("got users %v, want %v", have, want)
	}
}

func wantRepos(t *testing.T, got []*forgejo_sdk.Repository, want ...string) {
	t.Helper()
	have := make([]string, len(got))
	for i, r := range got {
		have[i] = r.Name
	}
	if fmt.Sprint(have) != fmt.Sprint(append([]string{}, want...)) {
		t.Fatalf("got repos %v, want %v", have, want)
	}
}
