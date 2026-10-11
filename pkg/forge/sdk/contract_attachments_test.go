// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/forgetest"
)

// attachmentHarness builds the real adapter against a fresh stub, with its
// raw-HTTP methods pointed at the same stub.
func attachmentHarness(t *testing.T) forgetest.Harness {
	stub := forgetest.NewStub(t)
	return forgetest.Harness{Port: NewAdapter(stub.NewClient(t)).WithRawBaseURL(stub.URL), Seed: stub.Fake}
}

func TestReleasesContract(t *testing.T) { forgetest.RunReleases(t, attachmentHarness) }

func TestAttachmentsContract(t *testing.T) { forgetest.RunAttachments(t, attachmentHarness) }
