// SPDX-License-Identifier: GPL-3.0-or-later

package forge

import (
	"context"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// Hooks is the repository-webhook domain of the port.
//
// Hooks carry their secret in Config; the port returns what Forgejo returns,
// and it is the tools' job never to pass it on.
//
// Failures from Forgejo are *StatusError.
type Hooks interface {
	ListRepoHooks(ctx context.Context, owner, repo string, opt forgejo_sdk.ListHooksOptions) (HookList, error)
	GetRepoHook(ctx context.Context, owner, repo string, id int64) (*forgejo_sdk.Hook, error)
	CreateRepoHook(ctx context.Context, owner, repo string, opt forgejo_sdk.CreateHookOption) (*forgejo_sdk.Hook, error)
	EditRepoHook(ctx context.Context, owner, repo string, id int64, opt forgejo_sdk.EditHookOption) error
	DeleteRepoHook(ctx context.Context, owner, repo string, id int64) error
	// TestRepoHook triggers a test delivery. The SDK has no call for it, so the
	// adapter reaches the endpoint over raw HTTP.
	TestRepoHook(ctx context.Context, owner, repo string, id int64) error
}

// HookList is one page of hooks plus the total Forgejo reports.
type HookList struct {
	Hooks []*forgejo_sdk.Hook
	// TotalCount is Forgejo's X-Total-Count, or nil when it did not say.
	TotalCount *int
}
