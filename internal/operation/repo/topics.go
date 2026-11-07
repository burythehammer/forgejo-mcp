// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"

	apprepo "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/repo"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/annotate"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/decode"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcpresult"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/params"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/pipeline"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/to"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	ListRepoTopicsToolName  = "list_repo_topics"
	SetRepoTopicsToolName   = "set_repo_topics"
	AddRepoTopicToolName    = "add_repo_topic"
	DeleteRepoTopicToolName = "delete_repo_topic"
)

var (
	ListRepoTopicsTool = mcp.NewTool(
		ListRepoTopicsToolName,
		annotate.Read(),
		mcp.WithDescription("List a repository's topics. Bounded by page (default 1) and limit (default 100); returns a response envelope {topics, page, limit, count, has_next}. has_next true means a further page may exist; re-issue the call with page incremented to fetch it."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(100), mcp.Min(1)),
	)

	SetRepoTopicsTool = mcp.NewTool(
		SetRepoTopicsToolName,
		annotate.Write(),
		mcp.WithDescription("Replace all repository topics (comma-separated). The topics key is required; an empty string clears every topic. Names are lowercased. At most 25 topics, each matching ^[a-z0-9][-.a-z0-9]*$ and at most 35 characters."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("topics", mcp.Required(), mcp.Description("Repository topics (comma-separated). Empty string clears all topics.")),
	)

	AddRepoTopicTool = mcp.NewTool(
		AddRepoTopicToolName,
		annotate.Write(),
		mcp.WithDescription("Add one topic to a repository. The name is lowercased and must match Forgejo's topic rules."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("topic", mcp.Required(), mcp.Description("Topic name")),
	)

	DeleteRepoTopicTool = mcp.NewTool(
		DeleteRepoTopicToolName,
		annotate.Destructive(),
		mcp.WithDescription("Delete one topic from a repository."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("topic", mcp.Required(), mcp.Description("Topic name")),
	)
)

type listRepoTopicsArgs struct {
	Owner string  `json:"owner" required:"true"`
	Repo  string  `json:"repo" required:"true"`
	Page  float64 `json:"page"`
	Limit float64 `json:"limit"`
}

// paging is the page and limit of the list. Validate runs it before the
// credential lookup.
func (a listRepoTopicsArgs) paging() (int, int, error) {
	return decode.PagingArgs(a.Page, a.Limit)
}

func (h handlers) ListRepoTopicsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return pipeline.Handle(h.adminService, listRepoTopics)(ctx, req)
}

func listRepoTopics(ctx context.Context, s *apprepo.AdminService, a listRepoTopicsArgs) (*mcp.CallToolResult, error) {
	pg, limit, err := a.paging()
	if err != nil {
		return mcpresult.Fail(err)
	}
	page, err := topicPage(s.ListRepoTopics(ctx, a.Owner, a.Repo, apprepo.AdminPaging{Page: pg, Limit: limit}))
	return mcpresult.Render(page, err)
}

type setRepoTopicsArgs struct {
	Owner  string  `json:"owner" required:"true"`
	Repo   string  `json:"repo" required:"true"`
	Topics *string `json:"topics" required:"true"`
}

func (h handlers) SetRepoTopicsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return pipeline.Handle(h.adminService, setRepoTopics)(ctx, req)
}

func setRepoTopics(ctx context.Context, s *apprepo.AdminService, a setRepoTopicsArgs) (*mcp.CallToolResult, error) {
	topics, err := s.SetRepoTopics(ctx, a.Owner, a.Repo, a.Topics)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return to.TextResult(map[string]any{"topics": topics})
}

type addRepoTopicArgs struct {
	Owner string `json:"owner" required:"true"`
	Repo  string `json:"repo" required:"true"`
	Topic string `json:"topic" required:"true"`
}

func (h handlers) AddRepoTopicFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return pipeline.Handle(h.adminService, addRepoTopic)(ctx, req)
}

func addRepoTopic(ctx context.Context, s *apprepo.AdminService, a addRepoTopicArgs) (*mcp.CallToolResult, error) {
	name, err := s.AddRepoTopic(ctx, a.Owner, a.Repo, a.Topic)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return to.TextResult(map[string]string{"topic": name, "status": "added"})
}

func (h handlers) DeleteRepoTopicFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return pipeline.Handle(h.adminService, deleteRepoTopic)(ctx, req)
}

func deleteRepoTopic(ctx context.Context, s *apprepo.AdminService, a addRepoTopicArgs) (*mcp.CallToolResult, error) {
	name, err := s.DeleteRepoTopic(ctx, a.Owner, a.Repo, a.Topic)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return to.TextResult(map[string]string{"topic": name, "status": "deleted"})
}

// Validate refuses a list call whose page or limit is out of range.
func (a listRepoTopicsArgs) Validate() error {
	_, _, err := a.paging()
	return err
}
