// SPDX-License-Identifier: GPL-3.0-or-later

package wiki

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
	appwiki "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/wiki"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/resource"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/wikiadapter"

	"github.com/mark3labs/mcp-go/mcp"
)

// decodeToolResult decodes the Result envelope a tool wraps its output in.
func decodeToolResult(res *mcp.CallToolResult, v any) error {
	if res == nil || len(res.Content) == 0 {
		return fmt.Errorf("empty tool result")
	}
	tc, ok := res.Content[0].(mcp.TextContent)
	if !ok {
		return fmt.Errorf("tool result is not text")
	}
	var env struct {
		Result json.RawMessage `json:"Result"`
	}
	if err := json.Unmarshal([]byte(tc.Text), &env); err != nil {
		return err
	}
	return json.Unmarshal(env.Result, v)
}

// wikiReq is a tool request for the test repository with extra arguments.
func wikiReq(args map[string]any) mcp.CallToolRequest {
	base := map[string]any{"owner": testOwner, "repo": testRepo}
	for k, v := range args {
		base[k] = v
	}
	return mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: base}}
}

// TestWikiReadParity_ToolAndResourceAgree drives get_wiki_page and
// get_wiki_revisions and the wiki page resource over the same fake page and
// history. The resource must carry the same title, name, commit, content and
// the first cap revisions the tools return.
func TestWikiReadParity_ToolAndResourceAgree(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	seedPage(t, f, "Hello", "# Hello\nfirst line\nsecond line")
	svc := appwiki.New(wikiadapter.New(f))
	for i := 0; i < resource.EmbeddedListCap+2; i++ {
		if _, err := svc.Update(context.Background(), appwiki.UpdateRequest{Owner: testOwner, Repo: testRepo, PageName: "Hello", Content: fmt.Sprintf("edit %d", i)}); err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()

	pageRes, err := h.GetWikiPageFn(ctx, wikiReq(map[string]any{"page_name": "Hello"}))
	if err != nil {
		t.Fatal(err)
	}
	var page model.WikiPageView
	if err := decodeToolResult(pageRes, &page); err != nil {
		t.Fatal(err)
	}

	revRes, err := h.GetWikiRevisionsFn(ctx, wikiReq(map[string]any{"page_name": "Hello", "page": float64(1), "limit": float64(resource.EmbeddedListCap + 2)}))
	if err != nil {
		t.Fatal(err)
	}
	var revs model.WikiRevisionsResult
	if err := decodeToolResult(revRes, &revs); err != nil {
		t.Fatal(err)
	}

	contents, err := readWiki(h, "Hello")
	if err != nil {
		t.Fatal(err)
	}
	payload := wikiPayload(t, contents)
	markdown := contents[1].(mcp.TextResourceContents).Text

	if payload.Title != page.Title || payload.PageName != page.PageName || payload.CommitSHA != page.CommitSHA {
		t.Errorf("resource title=%q page=%q sha=%q; tool title=%q page=%q sha=%q",
			payload.Title, payload.PageName, payload.CommitSHA, page.Title, page.PageName, page.CommitSHA)
	}
	if markdown != page.Content {
		t.Errorf("resource markdown differs from the page content the tool returns")
	}
	if !reflect.DeepEqual(payload.RecentRevisions, revs.Revisions[:resource.EmbeddedListCap]) {
		t.Errorf("resource revisions differ from the first %d the revisions tool returns", resource.EmbeddedListCap)
	}
	if !payload.Truncated || payload.ListTool != GetWikiRevisionsToolName {
		t.Errorf("resource truncated=%v list_tool=%q, want truncated pointing at %s", payload.Truncated, payload.ListTool, GetWikiRevisionsToolName)
	}
}
