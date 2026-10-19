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
	forgetest.RunPulls(t, func(t *testing.T) forgetest.Harness {
		stub := forgetest.NewStub(t)
		return forgetest.Harness{Port: NewAdapter(stub.NewClient(t)).WithRawBaseURL(stub.URL), Seed: stub.Fake}
	})
}

func TestOrgsContract(t *testing.T) {
	t.Parallel()
	forgetest.RunOrgs(t, harness)
}

func TestUsersContract(t *testing.T) {
	t.Parallel()
	forgetest.RunUsers(t, harness)
}

func TestRepoFilesContract(t *testing.T) {
	t.Parallel()
	forgetest.RunRepoFiles(t, func(t *testing.T) forgetest.Harness {
		stub := forgetest.NewStub(t)
		return forgetest.Harness{Port: NewAdapter(stub.NewClient(t)).WithRawBaseURL(stub.URL), Seed: stub.Fake}
	})
}

func TestActionsContract(t *testing.T) {
	t.Parallel()
	forgetest.RunActions(t, func(t *testing.T) forgetest.Harness {
		stub := forgetest.NewStub(t)
		return forgetest.Harness{Port: NewAdapter(stub.NewClient(t)).WithRawBaseURL(stub.URL), Seed: stub.Fake}
	})
}

func TestTrackingContract(t *testing.T) {
	t.Parallel()
	forgetest.RunTracking(t, func(t *testing.T) forgetest.Harness {
		stub := forgetest.NewStub(t)
		return forgetest.Harness{Port: NewAdapter(stub.NewClient(t)).WithRawBaseURL(stub.URL), Seed: stub.Fake}
	})
}

func TestSearchContract(t *testing.T) {
	t.Parallel()
	forgetest.RunSearch(t, func(t *testing.T) forgetest.Harness {
		stub := forgetest.NewStub(t)
		return forgetest.Harness{Port: NewAdapter(stub.NewClient(t)).WithRawBaseURL(stub.URL), Seed: stub.Fake}
	})
}

func TestIssuesContract(t *testing.T) {
	t.Parallel()
	forgetest.RunIssues(t, func(t *testing.T) forgetest.Harness {
		stub := forgetest.NewStub(t)
		return forgetest.Harness{Port: NewAdapter(stub.NewClient(t)).WithRawBaseURL(stub.URL), Seed: stub.Fake}
	})
}

func TestLabelsContract(t *testing.T) {
	t.Parallel()
	forgetest.RunLabels(t, func(t *testing.T) forgetest.Harness {
		stub := forgetest.NewStub(t)
		return forgetest.Harness{Port: NewAdapter(stub.NewClient(t)).WithRawBaseURL(stub.URL), Seed: stub.Fake}
	})
}

func TestDependenciesContract(t *testing.T) {
	t.Parallel()
	forgetest.RunDependencies(t, func(t *testing.T) forgetest.Harness {
		stub := forgetest.NewStub(t)
		return forgetest.Harness{Port: NewAdapter(stub.NewClient(t)).WithRawBaseURL(stub.URL), Seed: stub.Fake}
	})
}

func TestErrorsContract(t *testing.T) {
	t.Parallel()
	forgetest.RunErrors(t, harness)
}
