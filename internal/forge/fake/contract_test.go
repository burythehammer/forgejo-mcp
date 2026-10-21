// SPDX-License-Identifier: GPL-3.0-or-later

package fake_test

import (
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/forgetest"
)

func TestBranchProtectionsContract(t *testing.T) {
	t.Parallel()
	forgetest.RunBranchProtections(t, func(*testing.T) forgetest.Harness {
		f := fake.New()
		return forgetest.Harness{Port: f, Seed: f}
	})
}

func TestHooksContract(t *testing.T) {
	t.Parallel()
	forgetest.RunHooks(t, func(*testing.T) forgetest.Harness {
		f := fake.New()
		return forgetest.Harness{Port: f, Seed: f}
	})
}

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

func TestActionsContract(t *testing.T) {
	t.Parallel()
	forgetest.RunActions(t, func(*testing.T) forgetest.Harness {
		f := fake.New()
		return forgetest.Harness{Port: f, Seed: f}
	})
}

func TestTrackingContract(t *testing.T) {
	t.Parallel()
	forgetest.RunTracking(t, func(*testing.T) forgetest.Harness {
		f := fake.New()
		return forgetest.Harness{Port: f, Seed: f}
	})
}

func TestSearchContract(t *testing.T) {
	t.Parallel()
	forgetest.RunSearch(t, func(*testing.T) forgetest.Harness {
		f := fake.New()
		return forgetest.Harness{Port: f, Seed: f}
	})
}

func TestIssuesContract(t *testing.T) {
	t.Parallel()
	forgetest.RunIssues(t, func(*testing.T) forgetest.Harness {
		f := fake.New()
		return forgetest.Harness{Port: f, Seed: f}
	})
}

func TestLabelsContract(t *testing.T) {
	t.Parallel()
	forgetest.RunLabels(t, func(*testing.T) forgetest.Harness {
		f := fake.New()
		return forgetest.Harness{Port: f, Seed: f}
	})
}

func TestDependenciesContract(t *testing.T) {
	t.Parallel()
	forgetest.RunDependencies(t, func(*testing.T) forgetest.Harness {
		f := fake.New()
		return forgetest.Harness{Port: f, Seed: f}
	})
}

func TestErrorsContract(t *testing.T) {
	t.Parallel()
	forgetest.RunErrors(t, func(*testing.T) forgetest.Harness {
		f := fake.New()
		return forgetest.Harness{Port: f, Seed: f}
	})
}
