// SPDX-License-Identifier: GPL-3.0-or-later

package wiki

import (
	"context"

	appwiki "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/wiki"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/operation/decode"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/operation/mcpresult"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/operation/params"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge/wikiadapter"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	ListWikiPagesToolName    = "list_wiki_pages"
	GetWikiPageToolName      = "get_wiki_page"
	GetWikiRevisionsToolName = "get_wiki_revisions"
	CreateWikiPageToolName   = "create_wiki_page"
	UpdateWikiPageToolName   = "update_wiki_page"
	DeleteWikiPageToolName   = "delete_wiki_page"
	defaultLimit             = 30
)

var ListWikiPagesTool = mcp.NewTool(ListWikiPagesToolName,
	mcp.WithDescription("List wiki pages with page/limit pagination. Returns a response envelope {pages, page, limit, count, has_next}. has_next true means a further page may exist; re-issue the call with page incremented to fetch it."),
	mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
	mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
	mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
	mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(defaultLimit), mcp.Min(1)),
)

var GetWikiPageTool = mcp.NewTool(GetWikiPageToolName,
	mcp.WithDescription("Get one wiki page as decoded Markdown. Optional start_line/end_line bound content; total_lines is always returned."),
	mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
	mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
	mcp.WithString("page_name", mcp.Required(), mcp.Description(params.WikiPage)),
	mcp.WithNumber("start_line", mcp.Description("First line to return (1-based, inclusive)"), mcp.Min(1)),
	mcp.WithNumber("end_line", mcp.Description("Last line to return (1-based, inclusive)"), mcp.Min(1)),
)

var GetWikiRevisionsTool = mcp.NewTool(GetWikiRevisionsToolName,
	mcp.WithDescription("Get a wiki page's revision history with page/limit pagination; returns has_next and total_count, the page's total number of revisions as reported in the response body."),
	mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
	mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
	mcp.WithString("page_name", mcp.Required(), mcp.Description(params.WikiPage)),
	mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
	mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(defaultLimit), mcp.Min(1)),
)

var CreateWikiPageTool = mcp.NewTool(CreateWikiPageToolName,
	mcp.WithDescription("Create a wiki page. Slash-separated titles such as Parent/Child are a flat naming convention: Forgejo stores no parent-child relationship and does not create a parent page automatically. Creating an existing title overwrites it. Use the returned page_name verbatim; never derive it from title."),
	mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
	mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
	mcp.WithString("title", mcp.Required(), mcp.Description(params.WikiTitle)),
	mcp.WithString("content", mcp.Required(), mcp.Description(params.WikiContent)),
	mcp.WithString("message", mcp.Description(params.Message)),
)

var UpdateWikiPageTool = mcp.NewTool(UpdateWikiPageToolName,
	mcp.WithDescription("Update a wiki page. Writes are last-writer-wins; Forgejo provides no optimistic concurrency precondition."),
	mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
	mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
	mcp.WithString("page_name", mcp.Required(), mcp.Description(params.WikiPage)),
	mcp.WithString("title", mcp.Description(params.WikiTitle)),
	mcp.WithString("content", mcp.Required(), mcp.Description(params.WikiContent)),
	mcp.WithString("message", mcp.Description(params.Message)),
)

var DeleteWikiPageTool = mcp.NewTool(DeleteWikiPageToolName,
	mcp.WithDescription("Delete one wiki page by its server-normalized page_name."),
	mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
	mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
	mcp.WithString("page_name", mcp.Required(), mcp.Description(params.WikiPage)),
)

// RegisterTool registers the wiki tools. Each call takes its Forgejo port from
// f, so it runs under the credential of the request that made it.
func RegisterTool(s *server.MCPServer, f forge.Factory) {
	h := handlers{forge: f}
	s.AddTool(ListWikiPagesTool, h.ListWikiPagesFn)
	s.AddTool(GetWikiPageTool, h.GetWikiPageFn)
	s.AddTool(GetWikiRevisionsTool, h.GetWikiRevisionsFn)
	s.AddTool(CreateWikiPageTool, h.CreateWikiPageFn)
	s.AddTool(UpdateWikiPageTool, h.UpdateWikiPageFn)
	s.AddTool(DeleteWikiPageTool, h.DeleteWikiPageFn)
}

// handlers serve the wiki tools and resource. They hold the factory rather than
// a port so each request gets its own credential.
type handlers struct{ forge forge.Factory }

// Argument structs, one per tool. `json` names the MCP argument and
// `required:"true"` marks the required ones; mcptest.ArgsMatchSchema keeps
// them in agreement with the tool definitions. The numbers are tagged
// exact:"true": these tools have always read a number with a float64
// assertion, so a numeric string is ignored rather than read.

type listArgs struct {
	Owner string  `json:"owner" required:"true"`
	Repo  string  `json:"repo" required:"true"`
	Page  float64 `json:"page" exact:"true"`
	Limit float64 `json:"limit" exact:"true"`
}

type getArgs struct {
	Owner     string   `json:"owner" required:"true"`
	Repo      string   `json:"repo" required:"true"`
	PageName  string   `json:"page_name" required:"true"`
	StartLine *float64 `json:"start_line" exact:"true"`
	EndLine   *float64 `json:"end_line" exact:"true"`
}

type revisionsArgs struct {
	Owner    string  `json:"owner" required:"true"`
	Repo     string  `json:"repo" required:"true"`
	PageName string  `json:"page_name" required:"true"`
	Page     float64 `json:"page" exact:"true"`
	Limit    float64 `json:"limit" exact:"true"`
}

type createArgs struct {
	Owner   string `json:"owner" required:"true"`
	Repo    string `json:"repo" required:"true"`
	Title   string `json:"title" required:"true"`
	Content string `json:"content" required:"true"`
	Message string `json:"message"`
}

type updateArgs struct {
	Owner    string `json:"owner" required:"true"`
	Repo     string `json:"repo" required:"true"`
	PageName string `json:"page_name" required:"true"`
	Title    string `json:"title"`
	Content  string `json:"content" required:"true"`
	Message  string `json:"message"`
}

type pageArgs struct {
	Owner    string `json:"owner" required:"true"`
	Repo     string `json:"repo" required:"true"`
	PageName string `json:"page_name" required:"true"`
}

// run is the shared shape of a tool call: decode the arguments into A, take the
// use cases bound to this request's credential, and hand both to do, which
// runs one use case and renders its result.
func run[A any](ctx context.Context, h handlers, req mcp.CallToolRequest, name string,
	do func(context.Context, *appwiki.Service, A) (*mcp.CallToolResult, error),
) (*mcp.CallToolResult, error) {
	log.Debugf("Called %s", name)
	var args A
	if err := decode.Lax(req, &args); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	port, err := h.forge.For(ctx)
	if err != nil {
		return mcpresult.Fail(err)
	}
	return do(ctx, appwiki.New(wikiadapter.New(port)), args)
}

func (h handlers) ListWikiPagesFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "ListWikiPagesFn", func(ctx context.Context, s *appwiki.Service, a listArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render(s.List(ctx, appwiki.ListRequest{Owner: a.Owner, Repo: a.Repo, Page: a.Page, Limit: a.Limit}))
	})
}

func (h handlers) GetWikiPageFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "GetWikiPageFn", func(ctx context.Context, s *appwiki.Service, a getArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render(s.Get(ctx, appwiki.GetRequest{Owner: a.Owner, Repo: a.Repo, PageName: a.PageName, StartLine: a.StartLine, EndLine: a.EndLine}))
	})
}

func (h handlers) GetWikiRevisionsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "GetWikiRevisionsFn", func(ctx context.Context, s *appwiki.Service, a revisionsArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render(s.Revisions(ctx, appwiki.RevisionsRequest{Owner: a.Owner, Repo: a.Repo, PageName: a.PageName, Page: a.Page, Limit: a.Limit}))
	})
}

func (h handlers) CreateWikiPageFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "CreateWikiPageFn", func(ctx context.Context, s *appwiki.Service, a createArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render(s.Create(ctx, appwiki.CreateRequest{Owner: a.Owner, Repo: a.Repo, Title: a.Title, Content: a.Content, Message: a.Message}))
	})
}

func (h handlers) UpdateWikiPageFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "UpdateWikiPageFn", func(ctx context.Context, s *appwiki.Service, a updateArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render(s.Update(ctx, appwiki.UpdateRequest{
			Owner: a.Owner, Repo: a.Repo, PageName: a.PageName, Title: a.Title, Content: a.Content, Message: a.Message,
		}))
	})
}

func (h handlers) DeleteWikiPageFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "DeleteWikiPageFn", func(ctx context.Context, s *appwiki.Service, a pageArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render(s.Delete(ctx, appwiki.PageRef{Owner: a.Owner, Repo: a.Repo, PageName: a.PageName}))
	})
}
