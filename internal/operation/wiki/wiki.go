// SPDX-License-Identifier: GPL-3.0-or-later

package wiki

import (
	"context"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/numarg"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/list"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	appwiki "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/wiki"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/wikiadapter"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/log"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/annotate"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/decode"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcpresult"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/params"

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
	annotate.Read(),
	mcp.WithDescription("List wiki pages with page/limit pagination. Returns a response envelope {pages, page, limit, count, has_next}. has_next true means a further page may exist; re-issue the call with page incremented to fetch it."),
	mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
	mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
	mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
	mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(defaultLimit), mcp.Min(1)),
)

var GetWikiPageTool = mcp.NewTool(GetWikiPageToolName,
	annotate.Read(),
	mcp.WithDescription("Get one wiki page as decoded Markdown. Optional start_line/end_line bound content; total_lines is always returned."),
	mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
	mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
	mcp.WithString("page_name", mcp.Required(), mcp.Description(params.WikiPage)),
	mcp.WithNumber("start_line", mcp.Description("First line to return (1-based, inclusive)"), mcp.Min(1)),
	mcp.WithNumber("end_line", mcp.Description("Last line to return (1-based, inclusive)"), mcp.Min(1)),
)

var GetWikiRevisionsTool = mcp.NewTool(GetWikiRevisionsToolName,
	annotate.Read(),
	mcp.WithDescription("Get a wiki page's revision history with page/limit pagination; returns has_next and total_count, the page's total number of revisions as reported in the response body."),
	mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
	mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
	mcp.WithString("page_name", mcp.Required(), mcp.Description(params.WikiPage)),
	mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
	mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(defaultLimit), mcp.Min(1)),
)

var CreateWikiPageTool = mcp.NewTool(CreateWikiPageToolName,
	annotate.Write(),
	mcp.WithDescription("Create a wiki page. Slash-separated titles such as Parent/Child are a flat naming convention: Forgejo stores no parent-child relationship and does not create a parent page automatically. Creating an existing title overwrites it. Use the returned page_name verbatim; never derive it from title."),
	mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
	mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
	mcp.WithString("title", mcp.Required(), mcp.Description(params.WikiTitle)),
	mcp.WithString("content", mcp.Required(), mcp.Description(params.WikiContent)),
	mcp.WithString("message", mcp.Description(params.Message)),
)

var UpdateWikiPageTool = mcp.NewTool(UpdateWikiPageToolName,
	annotate.Write(),
	mcp.WithDescription("Update a wiki page. Writes are last-writer-wins; Forgejo provides no optimistic concurrency precondition."),
	mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
	mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
	mcp.WithString("page_name", mcp.Required(), mcp.Description(params.WikiPage)),
	mcp.WithString("title", mcp.Description(params.WikiTitle)),
	mcp.WithString("content", mcp.Required(), mcp.Description(params.WikiContent)),
	mcp.WithString("message", mcp.Description(params.Message)),
)

var DeleteWikiPageTool = mcp.NewTool(DeleteWikiPageToolName,
	annotate.Destructive(),
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
		pg, lim, perr := decode.PagingArgs(a.Page, a.Limit)
		if perr != nil {
			return mcpresult.Fail(perr)
		}
		return wikiPageList(s.List(ctx, appwiki.ListRequest{Owner: a.Owner, Repo: a.Repo, Page: pg, Limit: lim}))
	})
}

func (h handlers) GetWikiPageFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "GetWikiPageFn", func(ctx context.Context, s *appwiki.Service, a getArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render(s.Get(ctx, appwiki.GetRequest{Owner: a.Owner, Repo: a.Repo, PageName: a.PageName, StartLine: lineArg(a.StartLine), EndLine: lineArg(a.EndLine)}))
	})
}

func (h handlers) GetWikiRevisionsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "GetWikiRevisionsFn", func(ctx context.Context, s *appwiki.Service, a revisionsArgs) (*mcp.CallToolResult, error) {
		pg, lim, perr := decode.PagingArgs(a.Page, a.Limit)
		if perr != nil {
			return mcpresult.Fail(perr)
		}
		return wikiRevisionsResult(s.Revisions(ctx, appwiki.RevisionsRequest{Owner: a.Owner, Repo: a.Repo, PageName: a.PageName, Page: pg, Limit: lim}))
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

// wikiPageList renders one page of wiki pages in the list tool's wire form.
func wikiPageList(b list.Bounded[model.WikiPageSummary], err error) (*mcp.CallToolResult, error) {
	if err != nil {
		return mcpresult.Fail(err)
	}
	return mcpresult.Render(model.WikiPageListResult{Pages: b.Items, PageMeta: model.PageMetaOf(b)}, nil)
}

// wikiRevisionsResult renders one page of a page's history in the revisions tool's wire form.
func wikiRevisionsResult(b list.Bounded[model.WikiRevisionSummary], err error) (*mcp.CallToolResult, error) {
	if err != nil {
		return mcpresult.Fail(err)
	}
	total := 0
	if b.Total != nil {
		total = *b.Total
	}
	return mcpresult.Render(model.WikiRevisionsResult{Revisions: b.Items, Page: b.Page, HasNext: b.HasNext, TotalCount: total}, nil)
}

// lineArg reads an optional line number. A fraction is truncated, as these
// tools have always done, so the use case receives a whole number.
func lineArg(v *float64) *int {
	if v == nil {
		return nil
	}
	n := int(numarg.Truncate(*v))
	return &n
}
