// SPDX-License-Identifier: GPL-3.0-or-later

package forgetest

import (
	"errors"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge"
)

// RunParents runs the parent-scoped list contract against the implementation
// newHarness builds. A list under a parent that does not exist is not found;
// the same list under a parent that exists with nothing in it is empty. A 403
// or 5xx from Forgejo is never reported as not found, and a 403 whose message
// says "not found" is still not a miss: only the typed 404 status is.
//
// The list used is issue dependencies (the parent is an issue), which every
// Forgejo adapter serves.
func RunParents(t *testing.T, newHarness func(t *testing.T) Harness) {
	cases := []struct {
		name  string
		setup func(t *testing.T, h Harness)
		run   func(h Harness) error
		want  func(t *testing.T, err error)
	}{
		{"a missing parent's list is not found",
			func(t *testing.T, h Harness) { h.Seed.AddRepo(Owner, Repo) },
			func(h Harness) error {
				_, err := h.Port.ListIssueDependencies(ctx, Owner, Repo, 99, 1, 10)
				return err
			},
			func(t *testing.T, err error) { wantParentKind(t, err, forge.ErrNotFound, 404) },
		},
		{"a missing parent's dependents are not found",
			func(t *testing.T, h Harness) { h.Seed.AddRepo(Owner, Repo) },
			func(h Harness) error {
				_, err := h.Port.ListIssueDependents(ctx, Owner, Repo, 99, 1, 10)
				return err
			},
			func(t *testing.T, err error) { wantParentKind(t, err, forge.ErrNotFound, 404) },
		},
		{"a missing repository's list is not found",
			func(t *testing.T, h Harness) {},
			func(h Harness) error {
				_, err := h.Port.ListIssueDependencies(ctx, "nobody", "nothing", 1, 1, 10)
				return err
			},
			func(t *testing.T, err error) { wantParentKind(t, err, forge.ErrNotFound, 404) },
		},
		{"an existing parent with nothing in its list is an empty list",
			func(t *testing.T, h Harness) {
				h.Seed.AddRepo(Owner, Repo)
				newIssueIn(t, h, Owner, Repo, "lonely")
			},
			func(h Harness) error {
				deps, err := h.Port.ListIssueDependencies(ctx, Owner, Repo, 1, 1, 10)
				if err == nil && len(deps) != 0 {
					return errors.New("want an empty list, got entries")
				}
				return err
			},
			func(t *testing.T, err error) {
				if err != nil {
					t.Fatalf("want an empty list with no error, got %v (status %d)", err, forge.StatusCode(err))
				}
			},
		},
		{"a 403 on a parent's list is unauthorized, not not-found",
			func(t *testing.T, h Harness) {
				h.Seed.AddRepo(Owner, Repo)
				newIssueIn(t, h, Owner, Repo, "a")
				h.Seed.FailIssues(403)
			},
			func(h Harness) error {
				_, err := h.Port.ListIssueDependencies(ctx, Owner, Repo, 1, 1, 10)
				return err
			},
			func(t *testing.T, err error) { wantParentKind(t, err, forge.ErrUnauthorized, 403) },
		},
		{"a 500 on a parent's list is an upstream failure, not not-found",
			func(t *testing.T, h Harness) {
				h.Seed.AddRepo(Owner, Repo)
				newIssueIn(t, h, Owner, Repo, "a")
				h.Seed.FailIssues(500)
			},
			func(h Harness) error {
				_, err := h.Port.ListIssueDependencies(ctx, Owner, Repo, 1, 1, 10)
				return err
			},
			func(t *testing.T, err error) { wantParentKind(t, err, forge.ErrUpstream, 500) },
		},
		{"a 403 whose message says not found is still not a miss",
			func(t *testing.T, h Harness) {
				h.Seed.AddRepo(Owner, Repo)
				newIssueIn(t, h, Owner, Repo, "a")
				h.Seed.Refuse("ListIssueDependencies", 403, "issue not found")
			},
			func(h Harness) error {
				_, err := h.Port.ListIssueDependencies(ctx, Owner, Repo, 1, 1, 10)
				return err
			},
			func(t *testing.T, err error) { wantParentKind(t, err, forge.ErrUnauthorized, 403) },
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			c.setup(t, h)
			c.want(t, c.run(h))
		})
	}
}

// wantParentKind asserts err matches kind, carries status, and matches no other
// parent-list kind. A nil err fails: every case here is an answer or a failure.
func wantParentKind(t *testing.T, err error, kind error, status int) {
	t.Helper()
	if err == nil {
		t.Fatalf("want %v (status %d), got nil", kind, status)
	}
	if !errors.Is(err, kind) {
		t.Fatalf("got %v (status %d), want kind %v", err, forge.StatusCode(err), kind)
	}
	if forge.StatusCode(err) != status {
		t.Fatalf("got status %d, want %d (%v)", forge.StatusCode(err), status, err)
	}
	for _, other := range []error{forge.ErrNotFound, forge.ErrUnauthorized, forge.ErrValidation, forge.ErrUpstream} {
		if other != kind && errors.Is(err, other) { //nolint:errorlint // comparing the kinds themselves
			t.Fatalf("got %v, which also matches %v", err, other)
		}
	}
}
