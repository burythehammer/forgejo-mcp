// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/forgetest"
)

// The real adapter, against the shared stub server, must meet the same
// contract as the fake.
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

func TestOrgsContract(t *testing.T) {
	t.Parallel()
	forgetest.RunOrgs(t, func(t *testing.T) forgetest.Harness {
		stub := forgetest.NewStub(t)
		return forgetest.Harness{Port: NewAdapter(stub.NewClient(t)), Seed: stub.Fake}
	})
}

func TestUsersContract(t *testing.T) {
	t.Parallel()
	forgetest.RunUsers(t, func(t *testing.T) forgetest.Harness {
		stub := forgetest.NewStub(t)
		return forgetest.Harness{Port: NewAdapter(stub.NewClient(t)), Seed: stub.Fake}
	})
}
