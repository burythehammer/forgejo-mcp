// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge/forgetest"
)

// The real adapter, reading a refusal's JSON body off the stub, must meet the
// refusal contract the fake meets.
func TestRefusalsContract(t *testing.T) {
	t.Parallel()
	forgetest.RunRefusals(t, func(t *testing.T) forgetest.Harness {
		stub := forgetest.NewStub(t)
		return forgetest.Harness{Port: NewAdapter(stub.NewClient(t)).WithRawBaseURL(stub.URL), Seed: stub.Fake}
	})
}
