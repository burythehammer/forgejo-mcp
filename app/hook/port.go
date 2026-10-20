// SPDX-License-Identifier: GPL-3.0-or-later

// Package hook holds the repository webhook use cases behind the hook tools.
// Like all of app/ it is MCP-free and SDK-free (scripts/ci/check-app-boundary.sh):
// use cases take typed inputs, reach Forgejo only through Port, and return
// repo-owned types from app/model for an MCP adapter in operation/hook to
// render. Use cases return only SafeHook projections, never a secret.
package hook

import (
	"context"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/model"
)

// DefaultType is the webhook type used when the caller names none: the one
// type Forgejo supports for repository hooks.
const DefaultType = "forgejo"

// Port is the Forgejo webhook domain as the use cases need it, in repo-owned
// types. pkg/forge/hookadapter implements it over forge.Hooks. Failures from
// Forgejo come back as ordinary errors for the use case to wrap, except a
// Forgejo status failure on an edit, which comes back as a *StatusError.
type Port interface {
	// MaxResponseItems is the instance's pagination ceiling; ok is false when it
	// cannot be determined, which is "unknown", never a ceiling of zero.
	MaxResponseItems(ctx context.Context) (limit int, ok bool)
	ListRepoHooks(ctx context.Context, owner, repo string, p Paging) (model.HookList, error)
	GetRepoHook(ctx context.Context, owner, repo string, id int64) (*model.Hook, error)
	CreateRepoHook(ctx context.Context, owner, repo string, in CreateInput) (*model.Hook, error)
	EditRepoHook(ctx context.Context, owner, repo string, id int64, in EditInput) error
	DeleteRepoHook(ctx context.Context, owner, repo string, id int64) error
	TestRepoHook(ctx context.Context, owner, repo string, id int64) error
}

// Paging selects a page of a list.
type Paging struct{ Page, Limit int }

// CreateInput is a new webhook. The zero value of each field is what Forgejo
// gets when the caller did not set it.
type CreateInput struct {
	Type         string
	Config       map[string]string
	Events       []string
	BranchFilter string
	Active       bool
}

// EditInput is the part of a webhook the edit changes. Each field is sent only
// when set: a nil Config, Events, BranchFilter or Active leaves that part of
// the stored hook as it is, while a non-nil value is sent even when empty, so
// an empty BranchFilter or Events clears it. Config replaces the stored config,
// so the use case passes the merged map whenever one key changes.
type EditInput struct {
	Config       map[string]string
	Events       []string
	BranchFilter *string
	Active       *bool
}

// StatusError is a failure Forgejo answered with an HTTP status. Its Error is
// the underlying message, so wrapping it never alters what a tool reports.
type StatusError struct {
	Code int
	Err  error
}

func (e *StatusError) Error() string { return e.Err.Error() }
func (e *StatusError) Unwrap() error { return e.Err }
