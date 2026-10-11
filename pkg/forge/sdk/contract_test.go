// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/config"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/forgetest"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"
)

// The real adapter, against the shared stub server, must meet the same
// contract as the fake.
func harness(t *testing.T) forgetest.Harness {
	stub := forgetest.NewStub(t)
	return forgetest.Harness{Port: NewAdapter(stub.NewClient(t)), Seed: stub.Fake}
}

func TestRepoAdminContract(t *testing.T) {
	t.Parallel()
	forgetest.RunRepoAdmin(t, harness)
}

func TestPullsContract(t *testing.T) {
	t.Parallel()
	forgetest.RunPulls(t, harness)
}

func TestOrgsContract(t *testing.T) {
	t.Parallel()
	forgetest.RunOrgs(t, harness)
}

func TestUsersContract(t *testing.T) {
	t.Parallel()
	forgetest.RunUsers(t, harness)
}

// ChangeFiles goes over the package-level raw-HTTP target, so these cases point
// it at their stub and run one at a time (not parallel).
func TestRepoFilesContract(t *testing.T) {
	forgetest.RunRepoFiles(t, func(t *testing.T) forgetest.Harness {
		stub := forgetest.NewStub(t)
		forgejo.Configure(config.Forgejo{URL: stub.URL, Token: config.NewSecret("stub-token")}, "test")
		t.Cleanup(func() { forgejo.Configure(config.Forgejo{}, "test") })
		return forgetest.Harness{Port: NewAdapter(stub.NewClient(t)), Seed: stub.Fake}
	})
}
