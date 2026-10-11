// SPDX-License-Identifier: GPL-3.0-or-later

package forgetest

import (
	"fmt"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/fake"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// RunOrgs runs the organization contract (organizations, membership, teams)
// against the implementation that newHarness builds. newHarness is called once
// per case, so cases never share state and run in parallel.
func RunOrgs(t *testing.T, newHarness func(t *testing.T) Harness) {
	cases := []struct {
		name string
		run  func(t *testing.T, h Harness)
	}{
		{"get unknown organization is not found", func(t *testing.T, h Harness) {
			_, err := h.Port.GetOrg(ctx, "nothing")
			wantNotFound(t, err)
		}},
		{"created organization reads back", func(t *testing.T, h Harness) {
			created, err := h.Port.CreateOrg(ctx, forgejo_sdk.CreateOrgOption{
				Name: "acme", FullName: "Acme Inc", Description: "anvils", Website: "https://acme.test", Location: "desert",
			})
			must(t, err)
			if created.UserName != "acme" {
				t.Fatalf("created %+v", created)
			}
			got, err := h.Port.GetOrg(ctx, "acme")
			must(t, err)
			if got.UserName != "acme" || got.FullName != "Acme Inc" || got.Description != "anvils" ||
				got.Website != "https://acme.test" || got.Location != "desert" || got.Visibility != "public" {
				t.Fatalf("got %+v", got)
			}
		}},
		{"create without a name or with a bad visibility is rejected", func(t *testing.T, h Harness) {
			if _, err := h.Port.CreateOrg(ctx, forgejo_sdk.CreateOrgOption{}); err == nil {
				t.Fatal("want an error for an empty name")
			}
			if _, err := h.Port.CreateOrg(ctx, forgejo_sdk.CreateOrgOption{Name: "acme", Visibility: "secret"}); err == nil {
				t.Fatal("want an error for a bad visibility")
			}
		}},
		{"creating an existing organization is rejected", func(t *testing.T, h Harness) {
			newOrg(t, h, "acme")
			if _, err := h.Port.CreateOrg(ctx, forgejo_sdk.CreateOrgOption{Name: "acme"}); err == nil {
				t.Fatal("want an error")
			}
		}},
		{"edit changes the fields given and keeps the rest", func(t *testing.T, h Harness) {
			_, err := h.Port.CreateOrg(ctx, forgejo_sdk.CreateOrgOption{Name: "acme", FullName: "Acme", Description: "old"})
			must(t, err)
			must(t, h.Port.EditOrg(ctx, "acme", forgejo_sdk.EditOrgOption{Description: "new", Visibility: forgejo_sdk.VisibleTypePrivate}))
			got, err := h.Port.GetOrg(ctx, "acme")
			must(t, err)
			if got.Description != "new" || got.Visibility != "private" || got.FullName != "Acme" {
				t.Fatalf("got %+v", got)
			}
		}},
		{"edit unknown organization is not found", func(t *testing.T, h Harness) {
			wantNotFound(t, h.Port.EditOrg(ctx, "nothing", forgejo_sdk.EditOrgOption{Description: "x"}))
		}},
		{"deleted organization is gone", func(t *testing.T, h Harness) {
			newOrg(t, h, "acme")
			must(t, h.Port.DeleteOrg(ctx, "acme"))
			_, err := h.Port.GetOrg(ctx, "acme")
			wantNotFound(t, err)
			wantNotFound(t, h.Port.DeleteOrg(ctx, "acme"))
		}},
		{"the creator's organizations are listed and paginate", func(t *testing.T, h Harness) {
			for _, n := range []string{"a", "b", "c"} {
				newOrg(t, h, n)
			}
			all, err := h.Port.ListMyOrgs(ctx, forgejo_sdk.ListOrgsOptions{})
			must(t, err)
			wantOrgs(t, all, "a", "b", "c")

			p1, err := h.Port.ListMyOrgs(ctx, forgejo_sdk.ListOrgsOptions{ListOptions: forgejo_sdk.ListOptions{Page: 1, PageSize: 2}})
			must(t, err)
			wantOrgs(t, p1, "a", "b")
			p2, err := h.Port.ListMyOrgs(ctx, forgejo_sdk.ListOrgsOptions{ListOptions: forgejo_sdk.ListOptions{Page: 2, PageSize: 2}})
			must(t, err)
			wantOrgs(t, p2, "c")
		}},
		{"a user's organizations are the ones they belong to", func(t *testing.T, h Harness) {
			newOrg(t, h, "acme")
			newOrg(t, h, "other")
			must(t, h.Seed.AddOrgMember("acme", "bob"))
			got, err := h.Port.ListUserOrgs(ctx, "bob", forgejo_sdk.ListOrgsOptions{})
			must(t, err)
			wantOrgs(t, got, "acme")
			_, err = h.Port.ListUserOrgs(ctx, "nobody", forgejo_sdk.ListOrgsOptions{})
			wantNotFound(t, err)
		}},

		{"members list, check and remove", func(t *testing.T, h Harness) {
			newOrg(t, h, "acme")
			must(t, h.Seed.AddOrgMember("acme", "bob"))
			h.Seed.AddUser("carol")

			members, err := h.Port.ListOrgMembers(ctx, "acme", forgejo_sdk.ListOrgMembershipOption{})
			must(t, err)
			if got := userNames(members); fmt.Sprint(got) != fmt.Sprint([]string{fake.Poster, "bob"}) {
				t.Fatalf("members %v", got)
			}
			for user, want := range map[string]bool{"bob": true, "carol": false} {
				is, err := h.Port.CheckOrgMembership(ctx, "acme", user)
				must(t, err)
				if is != want {
					t.Fatalf("member(%s)=%v, want %v", user, is, want)
				}
			}

			must(t, h.Port.RemoveOrgMember(ctx, "acme", "bob"))
			is, err := h.Port.CheckOrgMembership(ctx, "acme", "bob")
			must(t, err)
			if is {
				t.Fatal("bob should have been removed")
			}
		}},
		{"members of an unknown organization are not found", func(t *testing.T, h Harness) {
			_, err := h.Port.ListOrgMembers(ctx, "nothing", forgejo_sdk.ListOrgMembershipOption{})
			wantNotFound(t, err)
			is, err := h.Port.CheckOrgMembership(ctx, "nothing", "bob")
			must(t, err)
			if is {
				t.Fatal("nobody is a member of an organization that does not exist")
			}
		}},
		{"removing an unknown user from an organization is not found", func(t *testing.T, h Harness) {
			newOrg(t, h, "acme")
			wantNotFound(t, h.Port.RemoveOrgMember(ctx, "acme", "nobody"))
			h.Seed.AddUser("bob")
			wantNotFound(t, h.Port.RemoveOrgMember(ctx, "nothing", "bob"))
		}},
		{"members paginate", func(t *testing.T, h Harness) {
			newOrg(t, h, "acme")
			for _, u := range []string{"u1", "u2", "u3"} {
				must(t, h.Seed.AddOrgMember("acme", u))
			}
			got, err := h.Port.ListOrgMembers(ctx, "acme", forgejo_sdk.ListOrgMembershipOption{ListOptions: forgejo_sdk.ListOptions{Page: 2, PageSize: 2}})
			must(t, err)
			if names := userNames(got); fmt.Sprint(names) != fmt.Sprint([]string{"u2", "u3"}) {
				t.Fatalf("page 2: %v", names)
			}
		}},

		{"created team is listed with its settings", func(t *testing.T, h Harness) {
			newOrg(t, h, "acme")
			team, err := h.Port.CreateTeam(ctx, "acme", forgejo_sdk.CreateTeamOption{
				Name: "devs", Description: "builders", Permission: forgejo_sdk.AccessModeWrite, CanCreateOrgRepo: true,
			})
			must(t, err)
			if team.ID == 0 || team.Name != "devs" || team.Permission != forgejo_sdk.AccessModeWrite || !team.CanCreateOrgRepo {
				t.Fatalf("created %+v", team)
			}
			teams, err := h.Port.ListOrgTeams(ctx, "acme", forgejo_sdk.ListTeamsOptions{})
			must(t, err)
			var found bool
			for _, tm := range teams {
				if tm.ID == team.ID {
					found = tm.Name == "devs" && tm.Description == "builders"
				}
			}
			if !found {
				t.Fatalf("created team missing from %+v", teams)
			}
		}},
		{"team creation is validated", func(t *testing.T, h Harness) {
			newOrg(t, h, "acme")
			if _, err := h.Port.CreateTeam(ctx, "acme", forgejo_sdk.CreateTeamOption{Permission: forgejo_sdk.AccessModeRead}); err == nil {
				t.Fatal("want an error for an empty name")
			}
			if _, err := h.Port.CreateTeam(ctx, "acme", forgejo_sdk.CreateTeamOption{Name: "x", Permission: "bogus"}); err == nil {
				t.Fatal("want an error for a bad permission")
			}
			_, err := h.Port.CreateTeam(ctx, "acme", forgejo_sdk.CreateTeamOption{Name: "devs", Permission: forgejo_sdk.AccessModeRead})
			must(t, err)
			if _, err := h.Port.CreateTeam(ctx, "acme", forgejo_sdk.CreateTeamOption{Name: "devs", Permission: forgejo_sdk.AccessModeRead}); err == nil {
				t.Fatal("want an error for a duplicate team name")
			}
		}},
		{"teams of an unknown organization are not found", func(t *testing.T, h Harness) {
			_, err := h.Port.ListOrgTeams(ctx, "nothing", forgejo_sdk.ListTeamsOptions{})
			wantNotFound(t, err)
			_, err = h.Port.CreateTeam(ctx, "nothing", forgejo_sdk.CreateTeamOption{Name: "devs", Permission: forgejo_sdk.AccessModeRead})
			wantNotFound(t, err)
		}},
		{"team members are added and removed", func(t *testing.T, h Harness) {
			newOrg(t, h, "acme")
			team := newTeam(t, h, "acme", "devs")
			h.Seed.AddUser("bob")
			must(t, h.Port.AddTeamMember(ctx, team.ID, "bob"))
			must(t, h.Port.AddTeamMember(ctx, team.ID, "bob")) // idempotent
			must(t, h.Port.RemoveTeamMember(ctx, team.ID, "bob"))
		}},
		{"team members of an unknown team or user are not found", func(t *testing.T, h Harness) {
			newOrg(t, h, "acme")
			team := newTeam(t, h, "acme", "devs")
			h.Seed.AddUser("bob")
			wantNotFound(t, h.Port.AddTeamMember(ctx, 9999, "bob"))
			wantNotFound(t, h.Port.AddTeamMember(ctx, team.ID, "nobody"))
			wantNotFound(t, h.Port.RemoveTeamMember(ctx, 9999, "bob"))
			wantNotFound(t, h.Port.RemoveTeamMember(ctx, team.ID, "nobody"))
		}},
		{"team repositories are added and removed", func(t *testing.T, h Harness) {
			newOrg(t, h, "acme")
			team := newTeam(t, h, "acme", "devs")
			h.Seed.AddRepo("acme", "anvil")
			must(t, h.Port.AddTeamRepository(ctx, team.ID, "acme", "anvil"))
			must(t, h.Port.RemoveTeamRepository(ctx, team.ID, "acme", "anvil"))
		}},
		{"team repositories of an unknown team are not found", func(t *testing.T, h Harness) {
			wantNotFound(t, h.Port.AddTeamRepository(ctx, 9999, "acme", "anvil"))
			wantNotFound(t, h.Port.RemoveTeamRepository(ctx, 9999, "acme", "anvil"))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.run(t, newHarness(t))
		})
	}
}

func newOrg(t *testing.T, h Harness, name string) {
	t.Helper()
	_, err := h.Port.CreateOrg(ctx, forgejo_sdk.CreateOrgOption{Name: name})
	must(t, err)
}

func newTeam(t *testing.T, h Harness, org, name string) *forgejo_sdk.Team {
	t.Helper()
	team, err := h.Port.CreateTeam(ctx, org, forgejo_sdk.CreateTeamOption{Name: name, Permission: forgejo_sdk.AccessModeRead})
	must(t, err)
	return team
}

func wantOrgs(t *testing.T, got []*forgejo_sdk.Organization, want ...string) {
	t.Helper()
	have := make([]string, len(got))
	for i, o := range got {
		have[i] = o.UserName
	}
	if fmt.Sprint(have) != fmt.Sprint(append([]string{}, want...)) {
		t.Fatalf("got organizations %v, want %v", have, want)
	}
}

func userNames(users []*forgejo_sdk.User) []string {
	out := make([]string, len(users))
	for i, u := range users {
		out[i] = u.UserName
	}
	return out
}
