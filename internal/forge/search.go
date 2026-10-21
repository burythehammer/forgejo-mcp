// SPDX-License-Identifier: GPL-3.0-or-later

package forge

import (
	"context"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// Search is the search domain of the port: users, organization teams and
// repositories by keyword. Failures from Forgejo are *StatusError.
type Search interface {
	SearchUsers(ctx context.Context, opt forgejo_sdk.SearchUsersOption) ([]*forgejo_sdk.User, error)
	SearchOrgTeams(ctx context.Context, org string, opt forgejo_sdk.SearchTeamsOptions) ([]*forgejo_sdk.Team, error)
	SearchRepos(ctx context.Context, opt forgejo_sdk.SearchRepoOptions) ([]*forgejo_sdk.Repository, error)
}
