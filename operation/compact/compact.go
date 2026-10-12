// SPDX-License-Identifier: GPL-3.0-or-later

// Package compact is the compact result that the write tools return by default.
// A write used to echo the whole entity, the body just sent and a roughly
// 25-field user object; the compact shape keeps what identifies the entity and
// what changed, and the optional `verbose` argument restores the full entity.
//
// One shape is defined per entity kind here, so every write tool that returns
// an issue or a pull request agrees on it. Read tools and resources keep the
// full entity.
package compact

import (
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/mcpresult"

	"github.com/mark3labs/mcp-go/mcp"
)

// VerboseDescription is the description of the optional `verbose` argument that
// the write tools declare.
const VerboseDescription = "Return the full entity instead of the compact summary (number, url, title, state, labels, assignees, updated_at). Default false."

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

// OfPullRequest returns the compact shape of a pull request.
func OfPullRequest(pr *model.PullRequest) PullRequest {
	out := PullRequest{Issue: Issue{
		Number:    pr.Index,
		URL:       pr.HTMLURL,
		Title:     pr.Title,
		State:     pr.State,
		Labels:    []string{},
		Assignees: []string{},
	}}
	if pr.Updated != nil {
		out.Updated = *pr.Updated
	}
	for _, l := range pr.Labels {
		if l != nil {
			out.Labels = append(out.Labels, l.Name)
		}
	}
	for _, a := range pr.Assignees {
		if a != nil {
			out.Assignees = append(out.Assignees, a.UserName)
		}
	}
	if pr.Head != nil {
		out.Head = pr.Head.Name
	}
	if pr.Base != nil {
		out.Base = pr.Base.Name
	}
	return out
}

// RenderIssue sends the result of an issue write: the use case's error, the
// full issue when verbose, or its compact shape.
func RenderIssue(iss *model.Issue, err error, verbose bool) (*mcp.CallToolResult, error) {
	if err != nil {
		return mcpresult.Fail(err)
	}
	if verbose {
		return mcpresult.Render(iss, nil)
	}
	return mcpresult.Render(OfIssue(iss), nil)
}

// RenderPullRequest is RenderIssue for a pull request write.
func RenderPullRequest(pr *model.PullRequest, err error, verbose bool) (*mcp.CallToolResult, error) {
	if err != nil {
		return mcpresult.Fail(err)
	}
	if verbose {
		return mcpresult.Render(pr, nil)
	}
	return mcpresult.Render(OfPullRequest(pr), nil)
}
