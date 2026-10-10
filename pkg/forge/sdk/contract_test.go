// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/forgetest"
)

// The real adapter, against the shared stub server, must meet the same
// contract as the fake.
func TestPullsContract(t *testing.T) {
	t.Parallel()
	forgetest.RunPulls(t, func(t *testing.T) forgetest.Harness {
		stub := forgetest.NewStub(t)
		return forgetest.Harness{Port: NewAdapter(stub.NewClient(t)), Seed: stub.Fake}
	})
}

func TestActionsContract(t *testing.T) {
	t.Parallel()
	forgetest.RunActions(t, func(t *testing.T) forgetest.Harness {
		stub := forgetest.NewStub(t)
		return forgetest.Harness{Port: NewAdapter(stub.NewClient(t)).WithBaseURL(stub.URL), Seed: stub.Fake}
	})
}

func TestTrackingContract(t *testing.T) {
	t.Parallel()
	forgetest.RunTracking(t, func(t *testing.T) forgetest.Harness {
		stub := forgetest.NewStub(t)
		return forgetest.Harness{Port: NewAdapter(stub.NewClient(t)).WithBaseURL(stub.URL), Seed: stub.Fake}
	})
}

func TestSearchContract(t *testing.T) {
	t.Parallel()
	forgetest.RunSearch(t, func(t *testing.T) forgetest.Harness {
		stub := forgetest.NewStub(t)
		return forgetest.Harness{Port: NewAdapter(stub.NewClient(t)).WithBaseURL(stub.URL), Seed: stub.Fake}
	})
}
