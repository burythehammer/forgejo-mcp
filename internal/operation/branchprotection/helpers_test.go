// SPDX-License-Identifier: GPL-3.0-or-later

package branchprotection

import (
	"context"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge/fake"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

const (
	testOwner = "goern"
	testRepo  = "forgejo-mcp"
)

// newFixture returns handlers served by a fresh fake that already has the
// repository testOwner/testRepo.
func newFixture(t *testing.T) (handlers, *fake.Forgejo) {
	t.Helper()
	f := fake.New()
	f.AddRepo(testOwner, testRepo)
	return handlers{forge: fake.Factory{Forgejo: f}}, f
}

// seedRule creates a rule through the fake's port.
func seedRule(t *testing.T, f *fake.Forgejo, opt forgejo_sdk.CreateBranchProtectionOption) {
	t.Helper()
	if _, err := f.CreateBranchProtection(context.Background(), testOwner, testRepo, opt); err != nil {
		t.Fatalf("seeding rule: %v", err)
	}
}
