// SPDX-License-Identifier: GPL-3.0-or-later

package forge

import (
	"context"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// Orgs is the organization domain of the port: organizations, their
// membership and their teams.
//
// Methods are named for what the org tools need, and use SDK types as plain
// data. Failures from Forgejo are *StatusError; validation failures (an empty
// organization name, an invalid visibility) are plain errors.
type Orgs interface {
	CreateOrg(ctx context.Context, opt forgejo_sdk.CreateOrgOption) (*forgejo_sdk.Organization, error)
	GetOrg(ctx context.Context, org string) (*forgejo_sdk.Organization, error)
	EditOrg(ctx context.Context, org string, opt forgejo_sdk.EditOrgOption) error
	DeleteOrg(ctx context.Context, org string) error
	// ListMyOrgs lists the organizations of the credential's own user.
	ListMyOrgs(ctx context.Context, opt forgejo_sdk.ListOrgsOptions) ([]*forgejo_sdk.Organization, error)
	ListUserOrgs(ctx context.Context, user string, opt forgejo_sdk.ListOrgsOptions) ([]*forgejo_sdk.Organization, error)

	ListOrgMembers(ctx context.Context, org string, opt forgejo_sdk.ListOrgMembershipOption) ([]*forgejo_sdk.User, error)
	// CheckOrgMembership reports whether user is a member of org. A user who
	// is not a member is false, not an error.
	CheckOrgMembership(ctx context.Context, org, user string) (bool, error)
	RemoveOrgMember(ctx context.Context, org, user string) error

	ListOrgTeams(ctx context.Context, org string, opt forgejo_sdk.ListTeamsOptions) ([]*forgejo_sdk.Team, error)
	CreateTeam(ctx context.Context, org string, opt forgejo_sdk.CreateTeamOption) (*forgejo_sdk.Team, error)
	AddTeamMember(ctx context.Context, teamID int64, user string) error
	RemoveTeamMember(ctx context.Context, teamID int64, user string) error
	AddTeamRepository(ctx context.Context, teamID int64, org, repo string) error
	RemoveTeamRepository(ctx context.Context, teamID int64, org, repo string) error
}
