// SPDX-License-Identifier: GPL-3.0-or-later

package pull

import (
	"context"
	"encoding/json"
	"fmt"

	apppull "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/pull"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/pulladapter"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/log"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/resource"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// resourceTimeFormat is the timestamp layout of the pull-request resource.
const resourceTimeFormat = "2006-01-02T15:04:05Z07:00"

// pullResourceURI is the forgejo:// URI of a pull request. The tool path names
// it as the subject of the resource error mapping that both paths share until #11.
func pullResourceURI(owner, repo string, index int64) string {
	return fmt.Sprintf("forgejo://repo/%s/%s/pr/%d", owner, repo, index)
}

// mapErr maps a port failure to the resource error, carrying the HTTP status
// into the message when Forgejo answered with one.
func mapErr(uri string, err error) error {
	return resource.MapForgejoError(uri, err)
}

// RegisterPullResources registers the forgejo://repo/{owner}/{repo}/pr/{index} resource template.
func RegisterPullResources(s *server.MCPServer, f forge.Factory) {
	h := handlers{forge: f}
	resource.RegisterTemplate(
		s,
		"forgejo://repo/{owner}/{repo}/pr/{index}",
		"Forgejo Pull Request",
		h.prResourceHandler,
		mcp.WithTemplateDescription(
			"Pull request metadata, head/base refs, mergeability, body sidecar, "+
				"and bounded recent comments and reviews. "+
				"URI: forgejo://repo/{owner}/{repo}/pr/{index}.",
		),
		mcp.WithTemplateMIMEType("application/json"),
	)
	log.Debug("Registered pull request resource template")
}

type prBranchRef struct {
	Label string `json:"label"`
	Ref   string `json:"ref"`
	SHA   string `json:"sha"`
}

type prCommentRef struct {
	ID          int64  `json:"id"`
	Author      string `json:"author"`
	CreatedAt   string `json:"created_at"`
	BodyExcerpt string `json:"body_excerpt"`
}

type prReviewRef struct {
	ID          int64  `json:"id"`
	Reviewer    string `json:"reviewer"`
	State       string `json:"state"`
	SubmittedAt string `json:"submitted_at"`
	BodyExcerpt string `json:"body_excerpt"`
}

type prResourcePayload struct {
	Owner     string `json:"owner"`
	Repo      string `json:"repo"`
	Index     int64  `json:"index"`
	Title     string `json:"title"`
	State     string `json:"state"` // "open", "closed", "merged"
	Author    string `json:"author"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	ClosedAt  string `json:"closed_at,omitempty"`
	MergedAt  string `json:"merged_at,omitempty"`
	Mergeable bool   `json:"mergeable"`

	Labels    []string `json:"labels"`
	Assignees []string `json:"assignees"`
	Milestone string   `json:"milestone,omitempty"`

	Head prBranchRef `json:"head"`
	Base prBranchRef `json:"base"`

	CommentCount int `json:"comment_count"`
	// ReviewCount is the pull request's review total. It is omitted when the
	// reviews were truncated and the server reported no total.
	ReviewCount *int `json:"review_count,omitempty"`

	RecentComments    []prCommentRef `json:"recent_comments"`
	CommentsTruncated bool           `json:"comments_truncated,omitempty"`
	CommentsListTool  string         `json:"comments_list_tool,omitempty"`

	RecentReviews    []prReviewRef `json:"recent_reviews"`
	ReviewsTruncated bool          `json:"reviews_truncated,omitempty"`
	ReviewsListTool  string        `json:"reviews_list_tool,omitempty"`
}

func (h handlers) prResourceHandler(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	uri := req.Params.URI
	params, err := resource.ParsePR(uri)
	if err != nil {
		return nil, resource.MapForgejoError(uri, err)
	}

	port, err := h.forge.For(ctx)
	if err != nil {
		return nil, fmt.Errorf("forgejo client: %w", err)
	}

	// The read use case fetches the pull request with its first EmbeddedListCap
	// comments and reviews, and one more of each so that a list over the cap is visible.
	read, err := apppull.New(pulladapter.New(port)).Read(ctx,
		apppull.PR{Owner: params.Owner, Repo: params.Repo, Index: params.Index},
		apppull.ReadOptions{CommentCap: resource.EmbeddedListCap, ReviewCap: resource.EmbeddedListCap},
	)
	if err != nil {
		return nil, mapErr(uri, err)
	}
	pr := read.PR

	// Comment and review bodies are excerpted here.
	commentRefs := make([]prCommentRef, len(read.Comments))
	for i, c := range read.Comments {
		author := ""
		if c.Poster != nil {
			author = c.Poster.UserName
		}
		commentRefs[i] = prCommentRef{
			ID:          c.ID,
			Author:      author,
			CreatedAt:   c.Created.Format(resourceTimeFormat),
			BodyExcerpt: resource.Excerpt(c.Body, 200),
		}
	}

	reviewRefs := make([]prReviewRef, len(read.Reviews))
	for i, r := range read.Reviews {
		reviewer := ""
		if r.Reviewer != nil {
			reviewer = r.Reviewer.UserName
		}
		reviewRefs[i] = prReviewRef{
			ID:          r.ID,
			Reviewer:    reviewer,
			State:       r.State,
			SubmittedAt: r.Submitted.Format(resourceTimeFormat),
			BodyExcerpt: resource.Excerpt(r.Body, 200),
		}
	}

	// compute state string
	state := pr.State
	if pr.HasMerged {
		state = "merged"
	}

	author := ""
	if pr.Poster != nil {
		author = pr.Poster.UserName
	}

	labels := make([]string, 0, len(pr.Labels))
	for _, l := range pr.Labels {
		labels = append(labels, l.Name)
	}
	assignees := make([]string, 0, len(pr.Assignees))
	for _, a := range pr.Assignees {
		assignees = append(assignees, a.UserName)
	}
	milestone := ""
	if pr.Milestone != nil {
		milestone = pr.Milestone.Title
	}

	createdAt, updatedAt, closedAt, mergedAt := "", "", "", ""
	if pr.Created != nil {
		createdAt = pr.Created.Format(resourceTimeFormat)
	}
	if pr.Updated != nil {
		updatedAt = pr.Updated.Format(resourceTimeFormat)
	}
	if pr.Closed != nil {
		closedAt = pr.Closed.Format(resourceTimeFormat)
	}
	if pr.Merged != nil {
		mergedAt = pr.Merged.Format(resourceTimeFormat)
	}

	var reviewCount *int
	if read.ReviewsTotalKnown {
		n := read.ReviewsTotal
		reviewCount = &n
	}

	headRef, baseRef := prBranchRef{}, prBranchRef{}
	if pr.Head != nil {
		headRef = prBranchRef{Label: pr.Head.Name, Ref: pr.Head.Ref, SHA: pr.Head.Sha}
	}
	if pr.Base != nil {
		baseRef = prBranchRef{Label: pr.Base.Name, Ref: pr.Base.Ref, SHA: pr.Base.Sha}
	}

	payload := prResourcePayload{
		Owner:             params.Owner,
		Repo:              params.Repo,
		Index:             params.Index,
		Title:             pr.Title,
		State:             state,
		Author:            author,
		CreatedAt:         createdAt,
		UpdatedAt:         updatedAt,
		ClosedAt:          closedAt,
		MergedAt:          mergedAt,
		Mergeable:         pr.Mergeable,
		Labels:            labels,
		Assignees:         assignees,
		Milestone:         milestone,
		Head:              headRef,
		Base:              baseRef,
		CommentCount:      pr.Comments,
		ReviewCount:       reviewCount,
		RecentComments:    commentRefs,
		CommentsTruncated: read.CommentsTruncated,
		RecentReviews:     reviewRefs,
		ReviewsTruncated:  read.ReviewsTruncated,
	}
	if read.CommentsTruncated {
		payload.CommentsListTool = resource.ToolRef("list_issue_comments")
	}
	if read.ReviewsTruncated {
		payload.ReviewsListTool = resource.ToolRef("list_pull_reviews")
	}

	jsonBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal PR payload: %w", err)
	}

	mdSidecar := fmt.Sprintf("# %s\n%s · #%d · %s · %s\nhead: %s · base: %s\n\n%s",
		pr.Title, state, params.Index, author, createdAt,
		headRef.Label, baseRef.Label, pr.Body)

	return []mcp.ResourceContents{
		mcp.TextResourceContents{URI: uri, MIMEType: "application/json", Text: string(jsonBytes)},
		mcp.TextResourceContents{URI: uri, MIMEType: "text/markdown", Text: mdSidecar},
	}, nil
}
