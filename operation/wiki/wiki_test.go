// SPDX-License-Identifier: GPL-3.0-or-later

package wiki

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/fake"

	"github.com/mark3labs/mcp-go/mcp"
)

// Handler tests run against the in-memory fake, build their own, and so run in
// parallel with no shared state.

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

func wikiRequest(args map[string]any) mcp.CallToolRequest {
	base := map[string]any{"owner": testOwner, "repo": testRepo}
	for k, v := range args {
		base[k] = v
	}
	return mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: base}}
}

func toolText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	content, ok := mcp.AsTextContent(result.Content[0])
	if !ok {
		t.Fatal("not text content")
	}
	return content.Text
}

// noTotal is a port whose listing carries no X-Total-Count, as on a Forgejo or
// proxy that omits the header.
type noTotal struct{ *fake.Forgejo }

func (n noTotal) ListWikiPages(ctx context.Context, owner, repo string, page, limit int) (forge.WikiPageList, error) {
	l, err := n.Forgejo.ListWikiPages(ctx, owner, repo, page, limit)
	l.TotalCount = nil
	return l, err
}

func TestCreateWikiPageToolDocumentsSubpages(t *testing.T) {
	t.Parallel()
	description := CreateWikiPageTool.Description
	for _, required := range []string{"Parent/Child", "flat naming convention", "no parent-child relationship", "does not create a parent page automatically", "returned page_name"} {
		if !strings.Contains(description, required) {
			t.Fatalf("create tool description does not document %q: %s", required, description)
		}
	}
}

func TestGetWikiPageDecodesAndSlicesLines(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	seedPage(t, f, "T", "a\r\nb\r\n")
	result, err := h.GetWikiPageFn(context.Background(), wikiRequest(map[string]any{"page_name": "T", "start_line": float64(1), "end_line": float64(2)}))
	if err != nil {
		t.Fatal(err)
	}
	text := toolText(t, result)
	if !strings.Contains(text, `"total_lines":3`) || !strings.Contains(text, `a\r\nb\r`) {
		t.Fatalf("unexpected result: %s", text)
	}
}

func TestGetWikiPageTotalLinesMatchesForCRLFAndLF(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	seedPage(t, f, "Crlf", "a\r\nb\r\n")
	seedPage(t, f, "Lf", "a\nb\n")
	for _, name := range []string{"Crlf", "Lf"} {
		result, err := h.GetWikiPageFn(context.Background(), wikiRequest(map[string]any{"page_name": name}))
		if err != nil {
			t.Fatal(err)
		}
		if text := toolText(t, result); !strings.Contains(text, `"total_lines":3`) {
			t.Fatalf("%s did not report three lines: %s", name, text)
		}
	}
}

func TestListWikiPagesIsBoundedAndUsesStableShape(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	for _, title := range []string{"A", "B", "Guides/Child", "D", "E"} {
		seedPage(t, f, title, "must-not-leak")
	}
	// Sorted: A, B, D, E, Guides%2FChild. Page 2 of size 2 is D and E.
	result, err := h.ListWikiPagesFn(context.Background(), wikiRequest(map[string]any{"page": float64(2), "limit": float64(2)}))
	if err != nil {
		t.Fatal(err)
	}
	text := toolText(t, result)
	if !strings.Contains(text, `"page_name":"D"`) || !strings.Contains(text, `"page_name":"E"`) || !strings.Contains(text, `"has_next":true`) {
		t.Fatalf("unexpected result: %s", text)
	}
	if !strings.Contains(text, `"total_count":5`) {
		t.Fatalf("expected total_count: %s", text)
	}
	if strings.Contains(text, "content_base64") || strings.Contains(text, "must-not-leak") || strings.Contains(text, "Guides") {
		t.Fatalf("unbounded or unsafe result: %s", text)
	}
}

func TestListWikiPagesReportsSubpageNameVerbatim(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	seedPage(t, f, "Guides/Child", "x")
	result, err := h.ListWikiPagesFn(context.Background(), wikiRequest(nil))
	if err != nil {
		t.Fatal(err)
	}
	text := toolText(t, result)
	if !strings.Contains(text, `"page_name":"Guides%2FChild"`) || !strings.Contains(text, `"sub_url":"Guides%2FChild"`) || !strings.Contains(text, `"has_next":false`) {
		t.Fatalf("unexpected result: %s", text)
	}
}

func TestListWikiPagesOmitsTotalCountWhenNotReported(t *testing.T) {
	t.Parallel()
	_, f := newFixture(t)
	seedPage(t, f, "One", "x")
	h := handlers{forge: fake.Factory{Forgejo: noTotal{f}}}
	result, err := h.ListWikiPagesFn(context.Background(), wikiRequest(nil))
	if err != nil {
		t.Fatal(err)
	}
	if text := toolText(t, result); strings.Contains(text, "total_count") {
		t.Fatalf("expected total_count to be omitted, got %s", text)
	}
}

func TestListWikiPagesWithoutWikiReturnsEmptyArray(t *testing.T) {
	t.Parallel()
	h := handlers{forge: fake.Factory{Forgejo: fake.New()}}
	result, err := h.ListWikiPagesFn(context.Background(), wikiRequest(nil))
	if err != nil {
		t.Fatal(err)
	}
	if text := toolText(t, result); !strings.Contains(text, `"pages":[]`) {
		t.Fatalf("empty list must be [], got %s", text)
	}
}

func TestGetWikiPageRejectsInvertedRange(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	seedPage(t, f, "T", "a\nb")
	_, err := h.GetWikiPageFn(context.Background(), wikiRequest(map[string]any{"page_name": "T", "start_line": float64(2), "end_line": float64(1)}))
	if err == nil {
		t.Fatal("expected inverted range error")
	}
}

func TestGetWikiPageRejectsInvalidBase64(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	if _, err := f.CreateWikiPage(context.Background(), testOwner, testRepo, "T", "%%%", "seed"); err != nil {
		t.Fatal(err)
	}
	_, err := h.GetWikiPageFn(context.Background(), wikiRequest(map[string]any{"page_name": "T"}))
	if err == nil || !strings.Contains(err.Error(), "decode wiki page content") {
		t.Fatalf("expected explicit base64 error, got %v", err)
	}
}

func TestGetWikiPageUnknownIsNotFound(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	_, err := h.GetWikiPageFn(context.Background(), wikiRequest(map[string]any{"page_name": "Missing"}))
	if !forge.IsNotFound(err) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestGetWikiRevisionsIsBoundedAndFlattensAuthor(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	made := seedPage(t, f, "T", "one")
	if _, err := f.EditWikiPage(context.Background(), testOwner, testRepo, made.SubURL, "T", b64("two"), "two"); err != nil {
		t.Fatal(err)
	}
	result, err := h.GetWikiRevisionsFn(context.Background(), wikiRequest(map[string]any{"page_name": "T", "limit": float64(1)}))
	if err != nil {
		t.Fatal(err)
	}
	text := toolText(t, result)
	if !strings.Contains(text, fmt.Sprintf(`"author":"%s"`, fake.Poster)) || !strings.Contains(text, `"message":"two"`) || !strings.Contains(text, `"has_next":true`) || strings.Contains(text, `"message":"seed T"`) {
		t.Fatalf("unexpected result: %s", text)
	}
	// total_count is the page's whole history, not the window.
	if !strings.Contains(text, `"total_count":2`) {
		t.Fatalf("expected total_count from the response count: %s", text)
	}
}

func TestGetWikiRevisionsLastPageHasNoNext(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	seedPage(t, f, "T", "one")
	result, err := h.GetWikiRevisionsFn(context.Background(), wikiRequest(map[string]any{"page_name": "T", "limit": float64(1)}))
	if err != nil {
		t.Fatal(err)
	}
	if text := toolText(t, result); !strings.Contains(text, `"has_next":false`) || !strings.Contains(text, `"total_count":1`) {
		t.Fatalf("unexpected result: %s", text)
	}
}

func TestGetWikiRevisions404IsError(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	_, err := h.GetWikiRevisionsFn(context.Background(), wikiRequest(map[string]any{"page_name": "missing"}))
	if !forge.IsNotFound(err) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestCreateWikiPageReturnsSafeStableShape(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	result, err := h.CreateWikiPageFn(context.Background(), wikiRequest(map[string]any{"title": "My Page", "content": "secret markdown"}))
	if err != nil {
		t.Fatal(err)
	}
	text := toolText(t, result)
	if !strings.Contains(text, `"page_name":"My-Page"`) || !strings.Contains(text, `"commit_sha":"`) || strings.Contains(text, "content_base64") || strings.Contains(text, "secret") {
		t.Fatalf("unsafe or unstable result: %s", text)
	}
	got, err := f.GetWikiPage(context.Background(), testOwner, testRepo, "My-Page")
	if err != nil {
		t.Fatal(err)
	}
	if got.ContentBase64 != b64("secret markdown") {
		t.Fatalf("content was not stored base64-encoded: %q", got.ContentBase64)
	}
	revs, err := f.GetWikiPageRevisions(context.Background(), testOwner, testRepo, "My-Page", 1, 10)
	if err != nil || len(revs.Commits) != 1 || !strings.Contains(revs.Commits[0].Message, "My Page") {
		t.Fatalf("default commit message should name the page: %+v err=%v", revs, err)
	}
}

func TestUpdateWithoutTitleReadsAndPreservesTitle(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	seedPage(t, f, "Getting Started", "old")
	result, err := h.UpdateWikiPageFn(context.Background(), wikiRequest(map[string]any{"page_name": "Getting-Started", "content": "new"}))
	if err != nil {
		t.Fatal(err)
	}
	text := toolText(t, result)
	if !strings.Contains(text, `"title":"Getting Started"`) || !strings.Contains(text, `"page_name":"Getting-Started"`) || !strings.Contains(text, `"commit_sha":"`) || strings.Contains(text, "content_base64") {
		t.Fatalf("unsafe or unstable result: %s", text)
	}
	got, err := f.GetWikiPage(context.Background(), testOwner, testRepo, "Getting-Started")
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Getting Started" || got.ContentBase64 != b64("new") {
		t.Fatalf("title not preserved or content not updated: %+v", got)
	}
}

func TestUpdateUnknownPageIsError(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	_, err := h.UpdateWikiPageFn(context.Background(), wikiRequest(map[string]any{"page_name": "Missing", "content": "x"}))
	if !forge.IsNotFound(err) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestDeleteWikiPageReportsSuccess(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	seedPage(t, f, "Guides/Setup", "x")
	result, err := h.DeleteWikiPageFn(context.Background(), wikiRequest(map[string]any{"page_name": "Guides%2FSetup"}))
	if err != nil {
		t.Fatal(err)
	}
	if text := toolText(t, result); !strings.Contains(text, `"deleted":true`) || !strings.Contains(text, `"page_name":"Guides%2FSetup"`) {
		t.Fatalf("unexpected result: %s", text)
	}
	if _, err := f.GetWikiPage(context.Background(), testOwner, testRepo, "Guides%2FSetup"); !forge.IsNotFound(err) {
		t.Fatalf("page should be gone, got %v", err)
	}
}

func TestToolsRefuseWhenTheFactoryRefuses(t *testing.T) {
	t.Parallel()
	refuse := fmt.Errorf("no credential")
	h := handlers{forge: fake.Factory{Err: refuse}}
	_, err := h.ListWikiPagesFn(context.Background(), wikiRequest(nil))
	if err == nil || !strings.Contains(err.Error(), "no credential") {
		t.Fatalf("got %v, want the factory's error", err)
	}
}
