// SPDX-License-Identifier: GPL-3.0-or-later

package forge

import (
	"context"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// BranchProtections is the branch-protection domain of the port: a
// repository's protection rules, addressed by rule name.
//
// Failures from Forgejo are *StatusError.
type BranchProtections interface {
	ListBranchProtections(ctx context.Context, owner, repo string, opt forgejo_sdk.ListBranchProtectionsOptions) ([]*forgejo_sdk.BranchProtection, error)
	GetBranchProtection(ctx context.Context, owner, repo, rule string) (*forgejo_sdk.BranchProtection, error)
	CreateBranchProtection(ctx context.Context, owner, repo string, opt forgejo_sdk.CreateBranchProtectionOption) (*forgejo_sdk.BranchProtection, error)
	// EditBranchProtection changes only the fields opt sets (non-nil).
	EditBranchProtection(ctx context.Context, owner, repo, rule string, opt forgejo_sdk.EditBranchProtectionOption) (*forgejo_sdk.BranchProtection, error)
	DeleteBranchProtection(ctx context.Context, owner, repo, rule string) error
}
