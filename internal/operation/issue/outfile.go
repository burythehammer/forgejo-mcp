// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"errors"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/lean"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/compact"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcpresult"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/outfile"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/to"

	"github.com/mark3labs/mcp-go/mcp"
)

// outFileOpts are the outFile argument's description and lean clause, shared by
// every tool that takes outFile so the two tools say the same thing.
var outFileOpts = []mcp.PropertyOption{
	mcp.Description("Bare file name. Writes the whole listing (every page) as a JSON array into the forgejo-mcp temp dir and replies {path,count,keys}; an existing file is overwritten. A listing over 1,000 issues (the file-output ceiling) fails and writes nothing. Refused with page or limit, and on the sse and http transports."),
	lean.Keep("bare file name; whole listing to file, ceiling 1,000, no page/limit"),
}

// listToFile writes the whole listing that all returns to the named output
// file and replies with where it went. It is the one file-output path for
// list_repo_issues and search_issues. A network transport refuses it before
// any listing, since the file would land on the server, where the caller
// cannot read it.
func listToFile(ctx context.Context, name string, verbose bool, all func(context.Context) ([]*model.Issue, error)) (*mcp.CallToolResult, error) {
	if forge.Remote(ctx) {
		return to.ErrorResult(errors.New("outFile is refused on a network transport: the file would land on the server, where the caller cannot read it; use the stdio transport"))
	}
	issues, err := all(ctx)
	if err != nil {
		return mcpresult.Fail(err)
	}
	res, err := writeIssues(name, issues, verbose)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return mcpresult.Render(res, nil)
}

// writeIssues writes issues to the output file: the full issues when verbose,
// otherwise the compact rows list_repo_issues returns.
func writeIssues(name string, issues []*model.Issue, verbose bool) (outfile.Result, error) {
	if verbose {
		return outfile.Write(name, issues, func(i *model.Issue) int64 { return i.Index })
	}
	rows := make([]compact.Issue, 0, len(issues))
	for _, iss := range issues {
		rows = append(rows, compact.OfIssue(iss))
	}
	return outfile.Write(name, rows, func(c compact.Issue) int64 { return c.Number })
}
