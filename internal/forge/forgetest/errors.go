// SPDX-License-Identifier: GPL-3.0-or-later

package forgetest

import (
	"errors"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// RunErrors runs the domain error contract against the implementation that
// newHarness builds. Each kind is reached through the port alone: a missing
// thing, refused input, and an answer Forgejo fails with (FailIssues stands in
// for a broken or forbidden Forgejo on the issue service). Every case asserts
// the kind it reaches and that no other kind matches it.
func RunErrors(t *testing.T, newHarness func(t *testing.T) Harness) {
	cases := []struct {
		name string
		kind error
		run  func(t *testing.T, h Harness) error
	}{
		{"an unknown issue is not found", forge.ErrNotFound, func(t *testing.T, h Harness) error {
			h.Seed.AddRepo(Owner, Repo)
			_, err := h.Port.GetIssue(ctx, Owner, Repo, 99)
			return err
		}},
		{"an unknown repository is not found", forge.ErrNotFound, func(t *testing.T, h Harness) error {
			_, err := h.Port.GetIssue(ctx, "nobody", "nothing", 1)
			return err
		}},
		{"a create without a title is a validation failure", forge.ErrValidation, func(t *testing.T, h Harness) error {
			h.Seed.AddRepo(Owner, Repo)
			_, err := h.Port.CreateIssue(ctx, Owner, Repo, forgejo_sdk.CreateIssueOption{Body: "x"})
			return err
		}},
		{"a 401 from Forgejo is unauthorized", forge.ErrUnauthorized, func(t *testing.T, h Harness) error {
			h.Seed.AddRepo(Owner, Repo)
			h.Seed.FailIssues(401)
			_, err := h.Port.GetIssue(ctx, Owner, Repo, 1)
			return err
		}},
		{"a 403 from Forgejo is unauthorized", forge.ErrUnauthorized, func(t *testing.T, h Harness) error {
			h.Seed.AddRepo(Owner, Repo)
			h.Seed.FailIssues(403)
			_, err := h.Port.GetIssue(ctx, Owner, Repo, 1)
			return err
		}},
		{"a 5xx from Forgejo is an upstream failure", forge.ErrUpstream, func(t *testing.T, h Harness) error {
			h.Seed.AddRepo(Owner, Repo)
			h.Seed.FailIssues(502)
			_, err := h.Port.GetIssue(ctx, Owner, Repo, 1)
			return err
		}},
	}
	kinds := []error{forge.ErrNotFound, forge.ErrUnauthorized, forge.ErrValidation, forge.ErrUpstream}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t)
			err := c.run(t, h)
			if err == nil {
				t.Fatalf("want an error of kind %v, got nil", c.kind)
			}
			if !errors.Is(err, c.kind) {
				t.Fatalf("got %v (status %d), want kind %v", err, forge.StatusCode(err), c.kind)
			}
			for _, other := range kinds {
				if other != c.kind && errors.Is(err, other) { //nolint:errorlint // comparing the kinds themselves, not a wrapped error
					t.Fatalf("got %v, which also matches %v", err, other)
				}
			}
		})
	}
}
