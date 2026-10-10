// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/forgetest"
)

// The real adapter, against the shared stub server, must meet the same
// contract as the fake.
func TestBranchProtectionsContract(t *testing.T) {
	t.Parallel()
	forgetest.RunBranchProtections(t, func(t *testing.T) forgetest.Harness {
		stub := forgetest.NewStub(t)
		return forgetest.Harness{Port: NewAdapter(stub.NewClient(t)), Seed: stub.Fake}
	})
}

func TestHooksContract(t *testing.T) {
	t.Parallel()
	forgetest.RunHooks(t, func(t *testing.T) forgetest.Harness {
		stub := forgetest.NewStub(t)
		return forgetest.Harness{Port: NewAdapter(stub.NewClient(t)).WithRawBaseURL(stub.URL), Seed: stub.Fake}
	})
}

func TestRepoAdminContract(t *testing.T) {
	t.Parallel()
	forgetest.RunRepoAdmin(t, func(t *testing.T) forgetest.Harness {
		stub := forgetest.NewStub(t)
		return forgetest.Harness{Port: NewAdapter(stub.NewClient(t)), Seed: stub.Fake}
	})
}

func TestPullsContract(t *testing.T) {
	t.Parallel()
	forgetest.RunPulls(t, func(t *testing.T) forgetest.Harness {
		stub := forgetest.NewStub(t)
		return forgetest.Harness{Port: NewAdapter(stub.NewClient(t)), Seed: stub.Fake}
	})
}
