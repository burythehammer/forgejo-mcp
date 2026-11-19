// SPDX-License-Identifier: GPL-3.0-or-later

package forgetest

import (
	"errors"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// RunRefusals runs the refusal contract against the implementation newHarness
// builds. Each case has Forgejo decline one call with a status and a message
// (through Seeder.Refuse). The caller must see the status's kind, the status,
// and Forgejo's message, whether the answer came from the fake directly or
// from the SDK adapter reading the stub's JSON body. A refusal with no message
// keeps the status alone.
func RunRefusals(t *testing.T, newHarness func(t *testing.T) Harness) {
	cases := []struct {
		name    string
		op      string // the port method Forgejo refuses
		status  int
		message string // Forgejo's message; "" for an answer with no message
		kind    error
		run     func(h Harness) error
	}{
		{"a refused dispatch carries Forgejo's message", "DispatchWorkflow", 422, "ref does not exist", forge.ErrValidation,
			func(h Harness) error {
				return h.Port.DispatchWorkflow(ctx, Owner, Repo, "ci.yaml", forgejo_sdk.DispatchWorkflowOption{Ref: "nope"})
			}},
		{"a refused repository secret write carries Forgejo's message", "SetRepoActionSecret", 403, "secrets are restricted", forge.ErrUnauthorized,
			func(h Harness) error {
				return h.Port.SetRepoActionSecret(ctx, Owner, Repo, forgejo_sdk.CreateSecretOption{Name: "TOKEN", Data: "x"})
			}},
		{"a refused repository secret delete carries Forgejo's message", "DeleteRepoActionSecret", 404, "secret does not exist", forge.ErrNotFound,
			func(h Harness) error {
				return h.Port.DeleteRepoActionSecret(ctx, Owner, Repo, "TOKEN")
			}},
		{"a refused organization secret write carries Forgejo's message", "SetOrgActionSecret", 422, "secret data is empty", forge.ErrValidation,
			func(h Harness) error {
				return h.Port.SetOrgActionSecret(ctx, Owner, forgejo_sdk.CreateSecretOption{Name: "TOKEN", Data: "x"})
			}},
		{"a refused organization secret delete carries Forgejo's message", "DeleteOrgActionSecret", 403, "not an organization owner", forge.ErrUnauthorized,
			func(h Harness) error {
				return h.Port.DeleteOrgActionSecret(ctx, Owner, "TOKEN")
			}},
		{"a refused file delete carries Forgejo's message", "DeleteFile", 422, "sha does not match the file", forge.ErrValidation,
			func(h Harness) error {
				return h.Port.DeleteFile(ctx, Owner, Repo, "docs/a.md", forgejo_sdk.DeleteFileOptions{FileOptions: forgejo_sdk.FileOptions{Message: "rm"}, SHA: "0"})
			}},
		{"a refused membership check carries Forgejo's message", "CheckOrgMembership", 403, "members are hidden from you", forge.ErrUnauthorized,
			func(h Harness) error {
				_, err := h.Port.CheckOrgMembership(ctx, Owner, "octo")
				return err
			}},
		{"a refused branch delete carries Forgejo's message", "DeleteBranch", 403, "branch is protected", forge.ErrUnauthorized,
			func(h Harness) error {
				_, err := h.Port.DeleteBranch(ctx, Owner, Repo, "feat")
				return err
			}},
		{"a refusal with no message keeps the status alone", "DeleteBranch", 404, "", forge.ErrNotFound,
			func(h Harness) error {
				_, err := h.Port.DeleteBranch(ctx, Owner, Repo, "feat")
				return err
			}},
		{"a 500 whose message says not found is an upstream failure", "DeleteBranch", 500, "branch not found", forge.ErrUpstream,
			func(h Harness) error {
				_, err := h.Port.DeleteBranch(ctx, Owner, Repo, "feat")
				return err
			}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			h := newHarness(t)
			h.Seed.Refuse(c.op, c.status, c.message)
			err := c.run(h)
			if err == nil {
				t.Fatalf("%s: want a refusal, got nil", c.op)
			}
			if !errors.Is(err, c.kind) {
				t.Fatalf("%s: got %v (status %d), want kind %v", c.op, err, forge.StatusCode(err), c.kind)
			}
			if forge.StatusCode(err) != c.status {
				t.Fatalf("%s: got status %d, want %d (%v)", c.op, forge.StatusCode(err), c.status, err)
			}
			if !errors.Is(c.kind, forge.ErrNotFound) && forge.IsNotFound(err) {
				t.Fatalf("%s: got %v, which reads as not found; only a typed 404 may", c.op, err)
			}
			if c.message != "" && !strings.Contains(err.Error(), c.message) {
				t.Fatalf("%s: got %q, want Forgejo's message %q in it", c.op, err.Error(), c.message)
			}
		})
	}
}
