// SPDX-License-Identifier: GPL-3.0-or-later

package fake_test

import (
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge/forgetest"
)

func TestWikiContract(t *testing.T) {
	t.Parallel()
	forgetest.RunWiki(t, func(*testing.T) forgetest.Harness {
		f := fake.New()
		return forgetest.Harness{Port: f, Seed: f}
	})
}

func TestPackagesContract(t *testing.T) {
	t.Parallel()
	forgetest.RunPackages(t, func(*testing.T) forgetest.Harness {
		f := fake.New()
		return forgetest.Harness{Port: f, Seed: f}
	})
}
