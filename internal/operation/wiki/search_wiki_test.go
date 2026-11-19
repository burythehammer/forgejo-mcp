// SPDX-License-Identifier: GPL-3.0-or-later

package wiki

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/mcptest"

	"github.com/mark3labs/mcp-go/mcp"
)

// These tests pin search_wiki: its tool definition (schema snapshot), its
// argument struct, and its output and error text (golden). They run over the
// in-memory Forgejo fake, so the scan goes through the real port.

func TestSearchWikiSchemaSnapshot(t *testing.T) {
	t.Parallel()
	mcptest.SchemaSnapshot(t, SearchWikiTool)
}

func TestSearchWikiArgsMatchSchema(t *testing.T) {
	t.Parallel()
	mcptest.ArgsMatchSchema(t, SearchWikiTool, searchArgs{})
}

// brokenRead is a Forgejo whose wiki read fails for one page; every other
// call goes to the fake.
type brokenRead struct {
	forge.Forgejo
	name string
}

func (b brokenRead) GetWikiPage(ctx context.Context, owner, repo, pageName string) (*forge.WikiPage, error) {
	if pageName == b.name {
		return nil, fmt.Errorf("forgejo unavailable for %s", pageName)
	}
	return b.Forgejo.GetWikiPage(ctx, owner, repo, pageName)
}

// searchOutput runs search_wiki through the server it is registered on and
// returns the text a client would read. A nil query is left out.
func searchOutput(t *testing.T, h handlers, query any) string {
	t.Helper()
	args := map[string]any{"owner": testOwner, "repo": testRepo}
	if query != nil {
		args["query"] = query
	}
	res, err := callTool(context.Background(), h, SearchWikiToolName, mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: args}})
	switch {
	case err != nil:
		return "ERROR: " + err.Error()
	case res.IsError:
		return "RESULT-ERROR: " + res.Content[0].(mcp.TextContent).Text
	}
	return res.Content[0].(mcp.TextContent).Text
}

type searchCase struct {
	name  string
	seed  func(t *testing.T, f *fake.Forgejo)
	wrap  func(f *fake.Forgejo) forge.Forgejo
	query any
}

var searchCases = []searchCase{
	{name: "title_only", query: "PAGINATION", seed: func(t *testing.T, f *fake.Forgejo) {
		seedPage(t, f, "Pagination", "Cursors are opaque.")
	}},
	{name: "body_line", query: "cursor", seed: func(t *testing.T, f *fake.Forgejo) {
		seedPage(t, f, "Decisions", "First line.\nWe chose cursor paging.\nLast line.")
	}},
	{name: "several_hits_one_page", query: "wiki", seed: func(t *testing.T, f *fake.Forgejo) {
		seedPage(t, f, "Notes", "wiki one\nwiki two\nnothing\nwiki three")
	}},
	{name: "no_hits", query: "absent", seed: func(t *testing.T, f *fake.Forgejo) {
		seedPage(t, f, "Home", "text")
	}},
	{name: "hit_cap", query: "entry", seed: func(t *testing.T, f *fake.Forgejo) {
		seedPage(t, f, "Log", strings.Repeat("entry\n", searchHitCap+5))
	}},
	{name: "page_cap", query: "needle", seed: func(t *testing.T, f *fake.Forgejo) {
		for i := 1; i <= searchPageCap+1; i++ {
			body := "plain"
			if i == searchPageCap+1 {
				body = "needle"
			}
			seedPage(t, f, fmt.Sprintf("Page %03d", i), body)
		}
	}},
	{name: "no_wiki", query: "anything"},
	{name: "missing_query", query: nil},
	{name: "blank_query", query: "  "},
	{name: "oversized_page_skipped", query: "needle", seed: func(t *testing.T, f *fake.Forgejo) {
		seedPage(t, f, "Big", strings.Repeat("needle ", forge.MaxInlineDownloadBytes/7+1))
		seedPage(t, f, "Small", "needle here")
	}},
	{name: "unreadable_page_skipped", query: "needle",
		seed: func(t *testing.T, f *fake.Forgejo) {
			seedPage(t, f, "Broken", "needle")
			seedPage(t, f, "Fine", "needle")
		},
		wrap: func(f *fake.Forgejo) forge.Forgejo { return brokenRead{Forgejo: f, name: "Broken"} },
	},
}

// The caps the golden cases exercise. They match the values in
// internal/app/wiki, which the use-case tests pin against the constants.
const (
	searchHitCap  = 20
	searchPageCap = 100
)

func TestSearchWikiGolden(t *testing.T) {
	t.Parallel()
	for _, c := range searchCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			f := fake.New()
			f.AddRepo(testOwner, testRepo)
			if c.seed != nil {
				c.seed(t, f)
			}
			var port forge.Forgejo = f
			if c.wrap != nil {
				port = c.wrap(f)
			}
			h := handlers{forge: fake.Factory{Forgejo: port}}
			mcptest.Golden(t, "search_wiki_"+c.name, []byte(searchOutput(t, h, c.query)))
		})
	}
}
