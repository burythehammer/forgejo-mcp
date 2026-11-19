// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"errors"
	"fmt"
	"strings"

	apppissue "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/issue"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/model"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/lean"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/compact"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/mcpresult"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/outfile"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/pipeline"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/to"

	"github.com/mark3labs/mcp-go/mcp"
)

// outFileOpts are the outFile argument's description and lean clause, shared by
// every tool that takes outFile so the two tools say the same thing.
var outFileOpts = []mcp.PropertyOption{
	mcp.Description("Bare file name. Writes the whole listing (every page) as a JSON array into the forgejo-mcp temp dir and replies {path,count,keys}; an existing file is overwritten. A listing over 1,000 issues (the file-output ceiling) fails and writes nothing. Refused with page or limit, and on the sse and http transports."),
	lean.Keep("bare file name; whole listing, max 1,000, no page/limit"),
}

// withBlockersOpts are the with_blockers argument's description and lean clause.
// The description names the added requests: one dependency read per listed
// issue, more only when an issue has more blockers than one page.
var withBlockersOpts = []mcp.PropertyOption{
	mcp.Description("Adds blocked_by to each file row: the issues that block it, [] when none. A blocker in the listed repository is its issue number (an integer); a blocker in another repository is an owner/repo#n string. Valid only with outFile. Makes one extra Forgejo dependency request per listed issue (more only when an issue has more blockers than one page), so a long listing is slower. A failed read fails the call and writes no file."),
	lean.Keep("with outFile; adds blocked_by"),
}

// refuseOutFilePaging refuses page or limit sent with outFile, since outFile
// reads every page. It reads the raw request: the argument struct reads a page
// or limit of 0 as absent, so only the request says it was sent. An argument
// sent as null counts as absent, as it does everywhere else.
func refuseOutFilePaging(ctx context.Context) error {
	req, ok := pipeline.Raw(ctx)
	if !ok {
		return nil
	}
	args := req.GetArguments()
	if !sent(args, "outFile") {
		return nil
	}
	if sent(args, "page") {
		return errors.New("page cannot be used with outFile: outFile writes the whole listing")
	}
	if sent(args, "limit") {
		return errors.New("limit cannot be used with outFile: outFile writes the whole listing")
	}
	return nil
}

// sent reports whether a raw argument is present and not null.
func sent(args map[string]any, name string) bool {
	v, ok := args[name]
	return ok && v != nil
}

// listToFile writes the whole listing that all returns to the named output
// file and replies with where it went. It is the one file-output path for
// list_repo_issues and search_issues.
func listToFile(ctx context.Context, name string, verbose bool, all func(context.Context) ([]*model.Issue, error)) (*mcp.CallToolResult, error) {
	return writeListing(ctx, func() (outfile.Result, error) {
		issues, err := all(ctx)
		if err != nil {
			return outfile.Result{}, err
		}
		return writeIssues(name, issues, verbose)
	})
}

// listBlockedToFile is listToFile with each row's blockers: the file rows gain
// blocked_by. Every blocker is read before anything is written, so a failed
// read leaves no file.
func listBlockedToFile(ctx context.Context, name string, verbose bool, all func(context.Context) ([]apppissue.Blocked, error)) (*mcp.CallToolResult, error) {
	return writeListing(ctx, func() (outfile.Result, error) {
		blocked, err := all(ctx)
		if err != nil {
			return outfile.Result{}, err
		}
		return writeBlocked(name, blocked, verbose)
	})
}

// writeListing refuses a network transport before any listing, since the file
// would land on the server, where the caller cannot read it. Otherwise it runs
// write and renders its reply, or its error.
func writeListing(ctx context.Context, write func() (outfile.Result, error)) (*mcp.CallToolResult, error) {
	if forge.Remote(ctx) {
		return to.ErrorResult(errors.New("outFile is refused on a network transport: the file would land on the server, where the caller cannot read it; use the stdio transport"))
	}
	res, err := write()
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

// blockedRow is a compact file row with the issues that block it.
type blockedRow struct {
	compact.Issue
	BlockedBy []any `json:"blocked_by"`
}

// blockedFull is a verbose file row with the issues that block it.
type blockedFull struct {
	*model.Issue
	BlockedBy []any `json:"blocked_by"`
}

// writeBlocked writes each listed issue with its blocked_by, in the compact or
// verbose shape. It names every blocker before it writes, so a blocker that
// cannot be named fails the call and writes nothing.
func writeBlocked(name string, blocked []apppissue.Blocked, verbose bool) (outfile.Result, error) {
	if verbose {
		rows := make([]blockedFull, 0, len(blocked))
		for _, b := range blocked {
			by, err := blockerNames(b)
			if err != nil {
				return outfile.Result{}, err
			}
			rows = append(rows, blockedFull{Issue: b.Issue, BlockedBy: by})
		}
		return outfile.Write(name, rows, func(r blockedFull) int64 { return r.Index })
	}
	rows := make([]blockedRow, 0, len(blocked))
	for _, b := range blocked {
		by, err := blockerNames(b)
		if err != nil {
			return outfile.Result{}, err
		}
		rows = append(rows, blockedRow{Issue: compact.OfIssue(b.Issue), BlockedBy: by})
	}
	return outfile.Write(name, rows, func(r blockedRow) int64 { return r.Number })
}

// blockerNames returns the blockers of one listed issue as they are written:
// the bare issue number for a blocker in the repository the issue was listed
// from, and owner/repo#n for one in another repository. It is always an array.
func blockerNames(b apppissue.Blocked) ([]any, error) {
	out := make([]any, 0, len(b.BlockedBy))
	for _, dep := range b.BlockedBy {
		if dep.Repository == nil || dep.Repository.FullName == "" {
			return nil, fmt.Errorf("blocker #%d of issue #%d has no repository in its record; nothing was written", dep.Index, b.Issue.Index)
		}
		if strings.EqualFold(dep.Repository.FullName, b.Repo) {
			out = append(out, dep.Index)
			continue
		}
		out = append(out, fmt.Sprintf("%s#%d", dep.Repository.FullName, dep.Index))
	}
	return out, nil
}
