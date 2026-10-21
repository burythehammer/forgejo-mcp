// SPDX-License-Identifier: GPL-3.0-or-later

package wiki

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/resource"

	"github.com/mark3labs/mcp-go/mcp"
)

func wikiPayload(t *testing.T, contents []mcp.ResourceContents) wikiResourcePayload {
	t.Helper()
	jsonBlock, ok := contents[0].(mcp.TextResourceContents)
	if !ok {
		t.Fatalf("expected text JSON block, got %#v", contents[0])
	}
	var payload wikiResourcePayload
	if err := json.Unmarshal([]byte(jsonBlock.Text), &payload); err != nil {
		t.Fatalf("decode JSON block: %v", err)
	}
	return payload
}

func readWiki(h handlers, name string) ([]mcp.ResourceContents, error) {
	uri := fmt.Sprintf("forgejo://repo/%s/%s/wiki/%s", testOwner, testRepo, name)
	return h.wikiResourceHandler(context.Background(), mcp.ReadResourceRequest{Params: mcp.ReadResourceParams{URI: uri}})
}

func TestWikiResourceReturnsMetadataAndMarkdown(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	made := seedPage(t, f, "Hello", "# Hello\n")
	contents, err := readWiki(h, "Hello")
	if err != nil {
		t.Fatal(err)
	}
	if len(contents) != 2 {
		t.Fatalf("expected JSON and markdown, got %d blocks", len(contents))
	}
	payload := wikiPayload(t, contents)
	if len(payload.RecentRevisions) != 1 || payload.RecentRevisions[0].Author != fake.Poster {
		t.Fatalf("unexpected JSON payload: %#v", payload)
	}
	if payload.CommitSHA != made.LastCommit.SHA || payload.Truncated || payload.Sentinel != "" {
		t.Fatalf("unexpected metadata/bounding: %#v", payload)
	}
	markdown, ok := contents[1].(mcp.TextResourceContents)
	if !ok || markdown.Text != "# Hello\n" {
		t.Fatalf("unexpected markdown block: %#v", contents[1])
	}
}

// revisionsFail is a port whose revision history cannot be read.
type revisionsFail struct{ *fake.Forgejo }

func (revisionsFail) GetWikiPageRevisions(context.Context, string, string, string, int, int) (*forge.WikiRevisions, error) {
	return nil, errors.New("500 internal error")
}

func TestWikiResourceDegradesWhenRevisionsFail(t *testing.T) {
	t.Parallel()
	_, f := newFixture(t)
	seedPage(t, f, "Hello", "")
	h := handlers{forge: fake.Factory{Forgejo: revisionsFail{f}}}
	contents, err := readWiki(h, "Hello")
	if err != nil {
		t.Fatal(err)
	}
	payload := wikiPayload(t, contents)
	if payload.RecentRevisions == nil || len(payload.RecentRevisions) != 0 {
		t.Fatalf("expected degraded non-nil empty revisions: %#v", payload.RecentRevisions)
	}
	if payload.Truncated || payload.Sentinel != "" {
		t.Fatalf("degraded revisions must not claim truncation: %#v", payload)
	}
}

func TestWikiResourceBoundsRevisionsWithSharedSentinel(t *testing.T) {
	t.Parallel()
	const total = resource.EmbeddedListCap + 7
	h, f := newFixture(t)
	made := seedPage(t, f, "Hello", "v0")
	var last *forge.WikiPage
	for i := 1; i < total; i++ {
		var err error
		last, err = f.EditWikiPage(context.Background(), testOwner, testRepo, made.SubURL, "Hello", b64("v"+strconv.Itoa(i)), "edit")
		if err != nil {
			t.Fatal(err)
		}
	}
	contents, err := readWiki(h, "Hello")
	if err != nil {
		t.Fatal(err)
	}
	payload := wikiPayload(t, contents)
	if len(payload.RecentRevisions) != resource.EmbeddedListCap {
		t.Fatalf("expected %d revisions, got %d", resource.EmbeddedListCap, len(payload.RecentRevisions))
	}
	if !payload.Truncated || payload.RevisionCount != total || payload.ListTool != GetWikiRevisionsToolName {
		t.Fatalf("unexpected bounding metadata: %#v", payload)
	}
	if !strings.Contains(payload.Sentinel, "[truncated:") || !strings.Contains(payload.Sentinel, strconv.Itoa(total)) || !strings.Contains(payload.Sentinel, GetWikiRevisionsToolName) {
		t.Fatalf("unexpected sentinel: %q", payload.Sentinel)
	}
	if payload.CommitSHA != last.LastCommit.SHA {
		t.Fatalf("commit SHA must come from the page, got %q", payload.CommitSHA)
	}
}

func TestWikiResourceMapsPrimaryError(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	_, err := readWiki(h, "Missing")
	var resourceErr *resource.ResourceError
	if !errors.As(err, &resourceErr) || resourceErr.Code != -32003 {
		t.Fatalf("expected not-found resource error, got %v", err)
	}
}

func TestWikiResourceMapsCredentialRefusal(t *testing.T) {
	t.Parallel()
	h := handlers{forge: fake.Factory{Err: forge.ErrNoRequestToken}}
	_, err := readWiki(h, "Hello")
	var resourceErr *resource.ResourceError
	if !errors.As(err, &resourceErr) || resourceErr.Code != -32002 {
		t.Fatalf("expected access-denied resource error, got %v", err)
	}
}

func TestWikiResourceTruncatesMarkdownAtUTF8Boundary(t *testing.T) {
	t.Parallel()
	const marker = "\n\n[truncated: use get_wiki_page with start_line/end_line to retrieve the remainder.]"
	keep := forge.MaxInlineDownloadBytes - len(marker)
	// Put the nominal byte boundary in the middle of a multi-byte rune.
	content := strings.Repeat("a", keep-1) + "€" + strings.Repeat("z", len(marker)+10)
	h, f := newFixture(t)
	seedPage(t, f, "Hello", content)
	contents, err := readWiki(h, "Hello")
	if err != nil {
		t.Fatal(err)
	}
	markdown := contents[1].(mcp.TextResourceContents).Text
	if !utf8.ValidString(markdown) {
		t.Fatal("truncated markdown is not valid UTF-8")
	}
	if len(markdown) > forge.MaxInlineDownloadBytes || !strings.Contains(markdown, "[truncated:") {
		t.Fatalf("unexpected truncated markdown length=%d", len(markdown))
	}
	if !strings.Contains(markdown, GetWikiPageToolName) {
		t.Fatalf("truncation marker must name %s", GetWikiPageToolName)
	}
}
