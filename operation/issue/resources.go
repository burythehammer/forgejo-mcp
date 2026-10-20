// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	apppissue "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/issue"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/operation/resource"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge/issueadapter"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterIssueResources registers the issue and comment resource templates.
func RegisterIssueResources(s *server.MCPServer, f forge.Factory) {
	h := newHandlers(f)
	resource.RegisterTemplate(
		s,
		"forgejo://repo/{owner}/{repo}/issue/{index}",
		"Forgejo Issue",
		h.issueResourceHandler,
		mcp.WithTemplateDescription(
			"Single issue metadata, rendered body, and bounded recent comments. "+
				"URI: forgejo://repo/{owner}/{repo}/issue/{index}.",
		),
		mcp.WithTemplateMIMEType("application/json"),
	)
	resource.RegisterTemplate(
		s,
		"forgejo://repo/{owner}/{repo}/{kind}/{index}/comment/{id}",
		"Forgejo Comment",
		h.commentResourceHandler,
		mcp.WithTemplateDescription(
			"Single comment by id. kind ∈ {issue, pr}. "+
				"PR comments use the same Forgejo issue-comment API. "+
				"URI: forgejo://repo/{owner}/{repo}/{kind}/{index}/comment/{id}.",
		),
		mcp.WithTemplateMIMEType("application/json"),
	)
	// The {?…} expansion is load-bearing, not documentation: mcp-go matches a
	// read against an anchored regexp built from this exact string, so a
	// template registered bare matches only the bare URI and every
	// query-bearing read fails with "resource not found" before any handler
	// runs. The bound and filter parameters are the entire point of this
	// resource, so registering without them makes it unusable.
	//
	// That same regexp is why the description spells out %2F and %20 for the
	// labels filter: an RFC 6570 value does not admit a raw '/' or a space, so
	// ?labels=Kind/Feature fails to match the template and the read is
	// rejected as "resource not found" before repoIssuesResourceHandler runs.
	// '+' does not work either — it matches nothing and would reach the API as
	// a literal '+'. Our own parsing tolerates a raw slash and the REST API
	// accepts one; the client simply never gets that far. This applies to path
	// variables as much as query values (see the branch_protection/{rule}
	// template, whose rule names are branch patterns), and slash-shaped label
	// vocabularies (Kind/…, Status/…) make it the first thing a caller hits.
	//
	// The description also warns that upstream drops unrecognised label names
	// rather than matching nothing. Measured against this repo's own API
	// (state=all, 484 issues total):
	//
	//	labels=Kind%2FOpenSpec            ->  10
	//	labels=Kind%2FBug                 ->   8
	//	labels=Kind%2FOpenSpec,Kind%2FBug ->   0   (so names are AND-ed)
	//	labels=Kind%2FOpenSpec,zzz-nope   ->  10   (not 0 — zzz-nope is dropped)
	//	labels=zzz-nope                   -> 484   (the unfiltered set)
	//
	// Under AND semantics an unknown name would have to yield 0 if it were
	// treated as an unmatchable term; it yields the known name's own count
	// instead, which is what proves the name is discarded outright. A filter
	// whose names are all unknown therefore degenerates to no filter at all.
	// Note this is the per-repo endpoint; search_issues uses the cross-repo
	// search endpoint and documents OR semantics, which is not a contradiction.
	// We could only surface a dropped name by resolving it against the repo's
	// (and org's) label list before the query — an extra round trip per read
	// and a behaviour change, not a doc fix — hence the warning here.
	resource.RegisterTemplate(
		s,
		"forgejo://repo/{owner}/{repo}/issues{?state,labels,page,limit}",
		"Forgejo Issue List",
		h.repoIssuesResourceHandler,
		mcp.WithTemplateDescription(
			"Bounded list of repository issues as rows — index, title, state, labels, "+
				"assignees, milestone, comment count, timestamps — with no bodies. "+
				"Filters and bounds are client-controlled (state ∈ {open, closed, all}, "+
				"default open; labels comma-separated, encoding a literal '/' in a "+
				"label name as %2F and spaces as %20, without double-encoding — '+' "+
				"does not work — e.g. "+
				"labels=Kind%2FFeature,Status%2FNeed%20More%20Info; page/limit, ceiling "+
				strconv.Itoa(resource.EmbeddedListCap)+"). "+
				"Forgejo silently ignores a label name it does not recognise: if no "+
				"listed name matches, the result is the UNFILTERED set rather than an "+
				"empty one, so a typo reads as a successful wide answer. Verify names "+
				"against "+ListRepoLabelsToolName+" instead of trusting the row count. "+
				"Read forgejo://repo/{owner}/{repo}/issue/{index} for a body. "+
				"URI: forgejo://repo/{owner}/{repo}/issues{?state,labels,page,limit}. "+
				"Truncation sentinel: "+ListRepoIssuesToolName+".",
		),
		mcp.WithTemplateMIMEType("application/json"),
	)
	resource.RegisterTemplate(
		s,
		// See the note on the issue-list template above: the {?…} expansion is
		// what makes page/limit reachable at all.
		"forgejo://repo/{owner}/{repo}/{kind}/{index}/comments{?page,limit}",
		"Forgejo Comment Thread",
		h.issueCommentsResourceHandler,
		mcp.WithTemplateDescription(
			"Bounded list of comments with FULL bodies (the single-issue resource "+
				"excerpts them). kind ∈ {issue, pr}; PR comments use the same Forgejo "+
				"issue-comment API. page/limit client-controlled, ceiling "+
				strconv.Itoa(resource.EmbeddedListCap)+". "+
				"URI: forgejo://repo/{owner}/{repo}/{kind}/{index}/comments{?page,limit}. "+
				"Truncation sentinel: "+ListIssueCommentsToolName+".",
		),
		mcp.WithTemplateMIMEType("application/json"),
	)
	log.Debug("Registered issue resource templates")
}

// ---- issue handler ----

type commentRef struct {
	ID          int64  `json:"id"`
	Author      string `json:"author"`
	CreatedAt   string `json:"created_at"`
	BodyExcerpt string `json:"body_excerpt"`
}

type issueResourcePayload struct {
	Owner          string       `json:"owner"`
	Repo           string       `json:"repo"`
	Index          int64        `json:"index"`
	Title          string       `json:"title"`
	State          string       `json:"state"`
	Author         string       `json:"author"`
	CreatedAt      string       `json:"created_at"`
	UpdatedAt      string       `json:"updated_at"`
	ClosedAt       string       `json:"closed_at,omitempty"`
	DueDate        string       `json:"due_date,omitempty"`
	Ref            string       `json:"ref,omitempty"`
	Labels         []string     `json:"labels"`
	Assignees      []string     `json:"assignees"`
	Milestone      string       `json:"milestone,omitempty"`
	CommentCount   int          `json:"comment_count"`
	RecentComments []commentRef `json:"recent_comments"`
	Truncated      bool         `json:"truncated,omitempty"`
	ListTool       string       `json:"list_tool,omitempty"`
}

func (h handlers) issueResourceHandler(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := req.Params.URI
	params, err := resource.ParseIssue(uri)
	if err != nil {
		return nil, resource.MapForgejoError(uri, err)
	}

	port, err := h.forge.For(ctx)
	if err != nil {
		return nil, fmt.Errorf("forgejo client: %w", err)
	}

	// The read use case fetches the issue with its first EmbeddedListCap
	// comments, and one more so that a thread over the cap is visible.
	read, err := apppissue.New(issueadapter.New(port)).Read(ctx,
		apppissue.Target{Owner: params.Owner, Repo: params.Repo, Index: params.Index},
		apppissue.ReadOptions{CommentCap: resource.EmbeddedListCap},
	)
	if err != nil {
		return nil, mapErr(uri, err)
	}
	iss := read.Issue

	// Comment bodies are excerpted here. The comment-thread resource carries full bodies.
	refs := make([]commentRef, len(read.Comments))
	for i, c := range read.Comments {
		author := ""
		if c.Poster != nil {
			author = c.Poster.UserName
		}
		refs[i] = commentRef{
			ID:          c.ID,
			Author:      author,
			CreatedAt:   c.Created.Format(resourceTimeFormat),
			BodyExcerpt: resource.Excerpt(c.Body, 200),
		}
	}

	labels := make([]string, 0, len(iss.Labels))
	for _, l := range iss.Labels {
		labels = append(labels, l.Name)
	}
	assignees := make([]string, 0, len(iss.Assignees))
	for _, a := range iss.Assignees {
		assignees = append(assignees, a.UserName)
	}

	author := ""
	if iss.Poster != nil {
		author = iss.Poster.UserName
	}
	milestone := ""
	if iss.Milestone != nil {
		milestone = iss.Milestone.Title
	}
	closedAt := ""
	if iss.Closed != nil {
		closedAt = iss.Closed.Format(resourceTimeFormat)
	}
	dueDate := ""
	if iss.Deadline != nil {
		dueDate = iss.Deadline.Format(resourceTimeFormat)
	}

	payload := issueResourcePayload{
		Owner:          params.Owner,
		Repo:           params.Repo,
		Index:          params.Index,
		Title:          iss.Title,
		State:          iss.State,
		Author:         author,
		CreatedAt:      iss.Created.Format(resourceTimeFormat),
		UpdatedAt:      iss.Updated.Format(resourceTimeFormat),
		ClosedAt:       closedAt,
		DueDate:        dueDate,
		Ref:            iss.Ref,
		Labels:         labels,
		Assignees:      assignees,
		Milestone:      milestone,
		CommentCount:   iss.Comments,
		RecentComments: refs,
		Truncated:      read.CommentsTruncated,
	}
	if read.CommentsTruncated {
		payload.ListTool = ListIssueCommentsToolName
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal issue payload: %w", err)
	}

	mdSidecar := fmt.Sprintf("# %s\nState: %s · #%d · %s · %s\n\n%s",
		iss.Title, iss.State, params.Index, author, iss.Created.Format("2006-01-02"), iss.Body)

	return []mcp.ResourceContents{
		mcp.TextResourceContents{URI: uri, MIMEType: "application/json", Text: string(jsonBytes)},
		mcp.TextResourceContents{URI: uri, MIMEType: "text/markdown", Text: mdSidecar},
	}, nil
}

// ---- comment handler ----

type commentResourcePayload struct {
	Owner     string `json:"owner"`
	Repo      string `json:"repo"`
	Kind      string `json:"kind"`
	Index     int64  `json:"index"`
	ID        int64  `json:"id"`
	Author    string `json:"author"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	Body      string `json:"body"`
	HTMLURL   string `json:"html_url"`
}

func (h handlers) commentResourceHandler(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := req.Params.URI
	params, err := resource.ParseComment(uri)
	if err != nil {
		// unknown kind and other parse errors → -32602
		return nil, resource.MapForgejoError(uri, err)
	}

	fg, err := h.forge.For(ctx)
	if err != nil {
		return nil, fmt.Errorf("forgejo client: %w", err)
	}

	// Forgejo PR comments ARE issue comments under the same API; kind only
	// affects display context, not the fetch path.
	c, err := fg.GetIssueComment(ctx, params.Owner, params.Repo, params.ID)
	if err != nil {
		return nil, mapErr(uri, err)
	}

	author := ""
	if c.Poster != nil {
		author = c.Poster.UserName
	}

	payload := commentResourcePayload{
		Owner:     params.Owner,
		Repo:      params.Repo,
		Kind:      params.Kind,
		Index:     params.Index,
		ID:        c.ID,
		Author:    author,
		CreatedAt: c.Created.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: c.Updated.Format("2006-01-02T15:04:05Z07:00"),
		Body:      c.Body,
		HTMLURL:   c.HTMLURL,
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal comment payload: %w", err)
	}

	mdSidecar := fmt.Sprintf("%s commented on %s#%d:\n%s", author, params.Kind, params.Index, c.Body)

	return []mcp.ResourceContents{
		mcp.TextResourceContents{URI: uri, MIMEType: "application/json", Text: string(jsonBytes)},
		mcp.TextResourceContents{URI: uri, MIMEType: "text/markdown", Text: mdSidecar},
	}, nil
}
