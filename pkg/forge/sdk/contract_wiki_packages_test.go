// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/forgetest"
)

// stubHarness is the real adapter, with its raw-HTTP binding, against a fresh
// shared stub server.
func stubHarness(t *testing.T) forgetest.Harness {
	stub := forgetest.NewStub(t)
	return forgetest.Harness{
		Port: NewAdapter(stub.NewClient(t)).WithRawBaseURL(stub.URL),
		Seed: stub.Fake,
	}
}

func TestWikiContract(t *testing.T) {
	t.Parallel()
	forgetest.RunWiki(t, stubHarness)
}

func TestPackagesContract(t *testing.T) {
	t.Parallel()
	forgetest.RunPackages(t, stubHarness)
}
