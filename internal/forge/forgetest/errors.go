// SPDX-License-Identifier: GPL-3.0-or-later

package forgetest

import (
	"errors"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge"

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

// RunMissingVersusEmpty pins the decision every list under a parent makes, once,
// for each implementation. A missing parent is not found. An existing parent
// with nothing under it is an empty list, not not-found. A failure with any
// other status is never read as not-found, whatever its message says.
func RunMissingVersusEmpty(t *testing.T, newHarness func(t *testing.T) Harness) {
	t.Run("comments of a missing issue are not found", func(t *testing.T) {
		h := newHarness(t)
		h.Seed.AddRepo(Owner, Repo)
		_, _, err := h.Port.ListIssueComments(ctx, Owner, Repo, 99, forgejo_sdk.ListIssueCommentOptions{})
		wantNotFound(t, err)
	})

	t.Run("comments of an existing issue with none are an empty list", func(t *testing.T) {
		h := newHarness(t)
		iss := newIssue(t, h, "quiet")
		comments, _, err := h.Port.ListIssueComments(ctx, Owner, Repo, iss.Index, forgejo_sdk.ListIssueCommentOptions{})
		must(t, err)
		if len(comments) != 0 {
			t.Fatalf("got %d comments, want an empty list", len(comments))
		}
	})

	failures := []struct {
		name   string
		status int
		kind   error
	}{
		{"a 403 listing under a missing issue is unauthorized, not not-found", 403, forge.ErrUnauthorized},
		{"a 500 listing under a missing issue is an upstream failure, not not-found", 500, forge.ErrUpstream},
	}
	for _, c := range failures {
		t.Run(c.name, func(t *testing.T) {
			h := newHarness(t)
			h.Seed.AddRepo(Owner, Repo)
			h.Seed.FailIssues(c.status)
			_, _, err := h.Port.ListIssueComments(ctx, Owner, Repo, 99, forgejo_sdk.ListIssueCommentOptions{})
			if err == nil {
				t.Fatalf("want an error of kind %v, got nil", c.kind)
			}
			if !errors.Is(err, c.kind) {
				t.Fatalf("got %v (status %d), want kind %v", err, forge.StatusCode(err), c.kind)
			}
			if forge.IsNotFound(err) {
				t.Fatalf("got %v (status %d): a %d must not be read as not found", err, forge.StatusCode(err), c.status)
			}
		})
	}
}
