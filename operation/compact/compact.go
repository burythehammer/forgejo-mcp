// SPDX-License-Identifier: GPL-3.0-or-later

// Package compact is the compact result that the write tools return by default.
// A write used to echo the whole entity, the body just sent and a roughly
// 25-field user object; the compact shape keeps what identifies the entity and
// what changed, and the optional `verbose` argument restores the full entity.
//
// One shape is defined per entity kind here, so every write tool that returns
// an issue or a pull request agrees on it. get_issue_by_index and
// get_pull_request_by_index are the reads that return a compact result by
// default: IssueRead and PullRequestRead are the write shape plus the fields a
// status check needs. list_repo_issues and search_issues return an IssueList,
// one compact issue per element inside the unchanged paging envelope. Other read
// tools and the forgejo:// resources keep the full entity.
package compact

import (
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/mcpresult"

	"github.com/mark3labs/mcp-go/mcp"
)

// VerboseDescription is the description of the optional `verbose` argument that
// the write tools declare.
const VerboseDescription = "Return the full entity instead of the compact summary. Default false."

// Issue is the compact result of an issue write.
type Issue struct {
	Number    int64     `json:"number"`
	URL       string    `json:"url"`
	Title     string    `json:"title"`
	State     string    `json:"state"`
	Labels    []string  `json:"labels"`
	Assignees []string  `json:"assignees"`
	Updated   time.Time `json:"updated_at"`
}

// PullRequest is the compact result of a pull request write: the issue shape
// plus the head and base branch names.
type PullRequest struct {
	Issue
	Head string `json:"head"`
	Base string `json:"base"`
}

// IssueRead is the compact result of get_issue_by_index: the issue write shape
// plus the body, the close time, the branch ref and the comment count. The
// poster, the milestone and the nested repository are verbose-only.
type IssueRead struct {
	Issue
	Body     string     `json:"body"`
	Closed   *time.Time `json:"closed_at"`
	Ref      string     `json:"ref"`
	Comments int        `json:"comments"`
}

// PullRequestRead is the compact result of get_pull_request_by_index: the pull
// request write shape plus the body, the merge and close times, the merge
// commit, the head commit, the mergeable flag and the comment count. The
// poster, the merger, the milestone and the nested repositories are
// verbose-only.
type PullRequestRead struct {
	PullRequest
	Body           string     `json:"body"`
	Merged         bool       `json:"merged"`
	MergedAt       *time.Time `json:"merged_at"`
	Closed         *time.Time `json:"closed_at"`
	Mergeable      bool       `json:"mergeable"`
	MergeCommitSHA *string    `json:"merge_commit_sha"`
	HeadSHA        string     `json:"head_sha"`
	Comments       int        `json:"comments"`
}

// Comment is the compact result of a comment write: the comment's ID, its URL
// and when it was last updated. The body just sent is not echoed back.
type Comment struct {
	ID      int64     `json:"id"`
	URL     string    `json:"url"`
	Updated time.Time `json:"updated_at"`
}

// OfIssue returns the compact shape of an issue.
func OfIssue(iss *model.Issue) Issue {
	out := Issue{
		Number:    iss.Index,
		URL:       iss.HTMLURL,
		Title:     iss.Title,
		State:     iss.State,
		Labels:    []string{},
		Assignees: []string{},
		Updated:   iss.Updated,
	}
	for _, l := range iss.Labels {
		if l != nil {
			out.Labels = append(out.Labels, l.Name)
		}
	}
	for _, a := range iss.Assignees {
		if a != nil {
			out.Assignees = append(out.Assignees, a.UserName)
		}
	}
	return out
}

// OfPullRequest returns the compact shape of a pull request: the issue shape
// built by OfIssue, plus the head and base branch names.
func OfPullRequest(pr *model.PullRequest) PullRequest {
	iss := &model.Issue{
		Index:     pr.Index,
		HTMLURL:   pr.HTMLURL,
		Title:     pr.Title,
		State:     pr.State,
		Labels:    pr.Labels,
		Assignees: pr.Assignees,
	}
	if pr.Updated != nil {
		iss.Updated = *pr.Updated
	}
	out := PullRequest{Issue: OfIssue(iss)}
	if pr.Head != nil {
		out.Head = pr.Head.Name
	}
	if pr.Base != nil {
		out.Base = pr.Base.Name
	}
	return out
}

// OfIssueRead returns the compact read shape of an issue.
func OfIssueRead(iss *model.Issue) IssueRead {
	return IssueRead{
		Issue:    OfIssue(iss),
		Body:     iss.Body,
		Closed:   iss.Closed,
		Ref:      iss.Ref,
		Comments: iss.Comments,
	}
}

// OfPullRequestRead returns the compact read shape of a pull request.
func OfPullRequestRead(pr *model.PullRequest) PullRequestRead {
	out := PullRequestRead{
		PullRequest:    OfPullRequest(pr),
		Body:           pr.Body,
		Merged:         pr.HasMerged,
		MergedAt:       pr.Merged,
		Closed:         pr.Closed,
		Mergeable:      pr.Mergeable,
		MergeCommitSHA: pr.MergedCommitID,
		Comments:       pr.Comments,
	}
	if pr.Head != nil {
		out.HeadSHA = pr.Head.Sha
	}
	return out
}

// IssueList is the compact result of list_repo_issues and search_issues: one
// page whose issues are the compact issue shape. The envelope is the page's own
// model.PageMeta, unchanged, so paging code reads the same either way.
type IssueList struct {
	Issues []Issue `json:"issues"`
	model.PageMeta
}

// Map projects each element of a page to its compact shape. An empty page maps
// to [] rather than null, so the envelope still reads as a list.
func Map[T any, C any](in []*T, of func(*T) C) []C {
	out := make([]C, 0, len(in))
	for _, v := range in {
		out = append(out, of(v))
	}
	return out
}

// OfIssueList returns the compact shape of a list_repo_issues page.
func OfIssueList(p *model.IssuePage) IssueList {
	return IssueList{Issues: Map(p.Issues, OfIssue), PageMeta: p.PageMeta}
}

// OfIssueSearchList returns the compact shape of a search_issues page.
func OfIssueSearchList(p *model.IssueSearchPage) IssueList {
	return IssueList{Issues: Map(p.Issues, OfIssue), PageMeta: p.PageMeta}
}

// OfComment returns the compact shape of a comment.
func OfComment(c *model.Comment) Comment {
	return Comment{ID: c.ID, URL: c.HTMLURL, Updated: c.Updated}
}

// render sends a write's result: the use case's error, the full entity when
// verbose, or its compact shape. An error is never summarized, so summarize
// only ever sees a non-nil entity.
func render[M any, C any](m *M, err error, verbose bool, summarize func(*M) C) (*mcp.CallToolResult, error) {
	if err != nil || verbose {
		return mcpresult.Render(m, err)
	}
	return mcpresult.Render(summarize(m), nil)
}

// RenderIssue sends the result of an issue write.
func RenderIssue(iss *model.Issue, err error, verbose bool) (*mcp.CallToolResult, error) {
	return render(iss, err, verbose, OfIssue)
}

// RenderPullRequest sends the result of a pull request write.
func RenderPullRequest(pr *model.PullRequest, err error, verbose bool) (*mcp.CallToolResult, error) {
	return render(pr, err, verbose, OfPullRequest)
}

// RenderIssueRead sends the result of get_issue_by_index. The caller maps the
// read's error itself and calls this only with a non-nil issue.
func RenderIssueRead(iss *model.Issue, verbose bool) (*mcp.CallToolResult, error) {
	return render(iss, nil, verbose, OfIssueRead)
}

// RenderPullRequestRead sends the result of get_pull_request_by_index. The caller
// maps the read's error itself and calls this only with a non-nil pull request.
func RenderPullRequestRead(pr *model.PullRequest, verbose bool) (*mcp.CallToolResult, error) {
	return render(pr, nil, verbose, OfPullRequestRead)
}

// RenderIssueList sends the result of list_repo_issues: the compact page, or the
// full page when verbose. The caller maps the error itself.
func RenderIssueList(p model.IssuePage, err error, verbose bool) (*mcp.CallToolResult, error) {
	return render(&p, err, verbose, OfIssueList)
}

// RenderIssueSearchList sends the result of search_issues: the compact page, or
// the full page when verbose. The caller maps the error itself.
func RenderIssueSearchList(p model.IssueSearchPage, err error, verbose bool) (*mcp.CallToolResult, error) {
	return render(&p, err, verbose, OfIssueSearchList)
}

// RenderComment sends the result of a comment write.
func RenderComment(c *model.Comment, err error, verbose bool) (*mcp.CallToolResult, error) {
	return render(c, err, verbose, OfComment)
}
