// SPDX-License-Identifier: GPL-3.0-or-later

package wiki

import (
	"context"
	"errors"
	"strings"

	appwiki "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/wiki"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/lean"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/annotate"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcpresult"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/params"

	"github.com/mark3labs/mcp-go/mcp"
)

// SearchWikiToolName is the search_wiki tool's name.
const SearchWikiToolName = "search_wiki"

var SearchWikiTool = mcp.NewTool(SearchWikiToolName,
	annotate.Read(),
	mcp.WithDescription("Search a repository's wiki pages for a plain-text query, matched case-insensitively on each page's title and on each line of its body. Returns {hits, pages_scanned, pages_skipped, truncated}; a hit is {page_name, line, snippet} and a title match has no line. The search reads at most 100 pages and returns at most 20 hits, and a page over 1 MiB is skipped and counted in pages_skipped. truncated true means a cap stopped the search, so no hits does not prove absence. Pass a hit's page_name to get_wiki_page, with its line as start_line."),
	mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
	mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
	mcp.WithString("query", mcp.Required(), mcp.Description("Plain text to find, matched case-insensitively; not a pattern"), lean.Keep("plain text, case-insensitive")),
)

// searchArgs is the search_wiki argument struct.
type searchArgs struct {
	Owner string `json:"owner" required:"true"`
	Repo  string `json:"repo" required:"true"`
	Query string `json:"query" required:"true"`
}

// Validate checks the repository and the query before the credential lookup.
func (a searchArgs) Validate() error {
	if err := requireRepo(a.Owner, a.Repo); err != nil {
		return err
	}
	if strings.TrimSpace(a.Query) == "" {
		return errors.New("query is required")
	}
	return nil
}

func searchWikiPages(ctx context.Context, s *appwiki.Service, a searchArgs) (*mcp.CallToolResult, error) {
	return mcpresult.Render(s.Search(ctx, appwiki.SearchRequest{Owner: a.Owner, Repo: a.Repo, Query: a.Query}))
}
