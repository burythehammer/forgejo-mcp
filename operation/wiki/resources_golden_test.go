// SPDX-License-Identifier: GPL-3.0-or-later

package wiki

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/mcptest"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/resource"

	"github.com/mark3labs/mcp-go/mcp"
)

// TestWikiResourceGolden_RevisionCapAndMarkdownSidecar pins the wiki resource
// past its revision cap: the JSON block stops at 30 revisions with the shared
// sentinel, and the markdown sidecar is the page content.
func TestWikiResourceGolden_RevisionCapAndMarkdownSidecar(t *testing.T) {
	resource.SetRegisteredTools([]string{GetWikiRevisionsToolName})
	t.Cleanup(func() { resource.SetRegisteredTools(nil) })

	const total = resource.EmbeddedListCap + 7
	h, f := newFixture(t)
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	f.Now = func() time.Time { return at }
	made := seedPage(t, f, "Hello", "v0")
	for i := 1; i < total; i++ {
		if _, err := f.EditWikiPage(context.Background(), testOwner, testRepo, made.SubURL, "Hello", b64("v"+strconv.Itoa(i)), "edit"); err != nil {
			t.Fatal(err)
		}
	}
	contents, err := readWiki(h, "Hello")
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for _, c := range contents {
		b.WriteString(c.(mcp.TextResourceContents).Text)
		b.WriteString("\n---\n")
	}
	mcptest.Golden(t, "resource_wiki_revision_cap_sidecar", []byte(b.String()))
}
