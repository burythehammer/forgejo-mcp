// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/forgetest"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"
)

// attachmentHarness builds the real adapter against a fresh stub. Attachment
// bytes travel over the raw-HTTP helpers, whose target is process-global, so
// the harness points them at this stub; the contract runs sequentially for
// that reason (see forgetest.RunReleases) and these tests are not parallel.
func attachmentHarness(t *testing.T) forgetest.Harness {
	stub := forgetest.NewStub(t)
	forgejo.ConfigureForTesting(stub.URL, "stub-token", "")
	t.Cleanup(func() { forgejo.ConfigureForTesting("", "", "") })
	return forgetest.Harness{Port: NewAdapter(stub.NewClient(t)), Seed: stub.Fake}
}

func TestReleasesContract(t *testing.T) { forgetest.RunReleases(t, attachmentHarness) }

func TestAttachmentsContract(t *testing.T) { forgetest.RunAttachments(t, attachmentHarness) }
