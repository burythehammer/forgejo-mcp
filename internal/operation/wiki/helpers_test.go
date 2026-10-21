// SPDX-License-Identifier: GPL-3.0-or-later

package wiki

import (
	"context"
	"encoding/base64"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"
)

// The wiki resources still read through the forge port; these helpers serve
// their tests. The tools are pinned by golden_test.go.

const (
	testOwner = "o"
	testRepo  = "r"
)

// newFixture returns handlers served by a fresh fake that already has the
// repository testOwner/testRepo.
func newFixture(t *testing.T) (handlers, *fake.Forgejo) {
	t.Helper()
	f := fake.New()
	f.AddRepo(testOwner, testRepo)
	return handlers{forge: fake.Factory{Forgejo: f}}, f
}

func b64(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

// seedPage writes a page through the fake's port and returns what it answered.
func seedPage(t *testing.T, f forge.Forgejo, title, content string) *forge.WikiPage {
	t.Helper()
	p, err := f.CreateWikiPage(context.Background(), testOwner, testRepo, title, b64(content), "seed "+title)
	if err != nil {
		t.Fatalf("seeding wiki page: %v", err)
	}
	return p
}
