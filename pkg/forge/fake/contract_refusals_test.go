// SPDX-License-Identifier: GPL-3.0-or-later

package fake_test

import (
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge/forgetest"
)

func TestRefusalsContract(t *testing.T) {
	t.Parallel()
	forgetest.RunRefusals(t, func(*testing.T) forgetest.Harness {
		f := fake.New()
		return forgetest.Harness{Port: f, Seed: f}
	})
}
