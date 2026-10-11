// SPDX-License-Identifier: GPL-3.0-or-later

package fake_test

import (
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/fake"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/forgetest"
)

func harness(*testing.T) forgetest.Harness {
	f := fake.New()
	return forgetest.Harness{Port: f, Seed: f}
}

func TestRepoAdminContract(t *testing.T) {
	t.Parallel()
	forgetest.RunRepoAdmin(t, harness)
}

func TestPullsContract(t *testing.T) {
	t.Parallel()
	forgetest.RunPulls(t, harness)
}

func TestRepoFilesContract(t *testing.T) {
	t.Parallel()
	forgetest.RunRepoFiles(t, harness)
}

func TestOrgsContract(t *testing.T) {
	t.Parallel()
	forgetest.RunOrgs(t, harness)
}

func TestUsersContract(t *testing.T) {
	t.Parallel()
	forgetest.RunUsers(t, harness)
}
