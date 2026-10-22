// SPDX-License-Identifier: GPL-3.0-or-later

// Package hook provides MCP tools and resources to read and manage
// Forgejo/Codeberg repository webhooks (/repos/{owner}/{repo}/hooks).
package hook

import (
	"context"
	"fmt"

	apphook "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/hook"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/hookadapter"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/log"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/decode"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcpresult"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/params"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/to"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	ListRepoHooksToolName  = "list_repo_hooks"
	GetRepoHookToolName    = "get_repo_hook"
	CreateRepoHookToolName = "create_repo_hook"
	EditRepoHookToolName   = "edit_repo_hook"
	DeleteRepoHookToolName = "delete_repo_hook"
	TestRepoHookToolName   = "test_repo_hook"
)

var (
	ListRepoHooksTool = mcp.NewTool(
		ListRepoHooksToolName,
		mcp.WithDescription("List repository webhooks. page/limit control pagination (default: page 1, limit 30); a limit above this instance's max_response_items is refused, not clamped. Returns a response envelope {hooks, page, limit, count, total_count, has_next}; total_count appears only when Forgejo reports X-Total-Count. has_next true means a further page may exist; re-issue the call with page incremented to fetch it."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("page", mcp.Required(), mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("limit", mcp.Required(), mcp.Description(params.Limit), mcp.DefaultNumber(30), mcp.Min(1)),
	)

	GetRepoHookTool = mcp.NewTool(
		GetRepoHookToolName,
		mcp.WithDescription("Get a single repository webhook by ID"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Webhook ID, a positive whole number; fractions and zero or negative numbers are rejected")),
	)

	CreateRepoHookTool = mcp.NewTool(
		CreateRepoHookToolName,
		mcp.WithDescription("Create a repository webhook. The secret is accepted but never echoed in the response."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithString("url", mcp.Required(), mcp.Description("Payload URL for the webhook")),
		mcp.WithString("type", mcp.Description(`Webhook type (default "forgejo"; only "forgejo" is supported for repository hooks)`)),
		mcp.WithString("content_type", mcp.Description(`Content type: "json" or "form" (default "json")`)),
		mcp.WithString("secret", mcp.Description("HMAC secret for payload signing (write-only; not returned)")),
		mcp.WithString("http_method", mcp.Description(`HTTP method: "POST" or "GET" (default "POST")`)),
		mcp.WithString("branch_filter", mcp.Description("Branch filter glob (e.g. main, release/*)")),
		mcp.WithString("events", mcp.Description(`Webhook events (comma-separated, e.g. "push,pull_request")`)),
		mcp.WithBoolean("active", mcp.Description("Whether the webhook is active (default true)")),
	)

	EditRepoHookTool = mcp.NewTool(
		EditRepoHookToolName,
		mcp.WithDescription("Edit a repository webhook. Only fields you pass are changed; omitted fields are left untouched."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Webhook ID, a positive whole number; fractions and zero or negative numbers are rejected")),
		mcp.WithString("url", mcp.Description("New payload URL")),
		mcp.WithString("content_type", mcp.Description(`Content type: "json" or "form"`)),
		mcp.WithString("secret", mcp.Description("New HMAC secret (write-only; not returned)")),
		mcp.WithString("http_method", mcp.Description(`HTTP method: "POST" or "GET"`)),
		mcp.WithString("branch_filter", mcp.Description("Branch filter glob")),
		mcp.WithString("events", mcp.Description(`Webhook events (comma-separated)`)),
		mcp.WithBoolean("active", mcp.Description("Whether the webhook is active")),
	)

	DeleteRepoHookTool = mcp.NewTool(
		DeleteRepoHookToolName,
		mcp.WithDescription("Delete a repository webhook by ID"),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Webhook ID, a positive whole number; fractions and zero or negative numbers are rejected")),
	)

	TestRepoHookTool = mcp.NewTool(
		TestRepoHookToolName,
		mcp.WithDescription("Trigger a test delivery for a repository webhook. WARNING: each call triggers a live HTTP delivery to the webhook URL."),
		mcp.WithString("owner", mcp.Required(), mcp.Description(params.Owner)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Webhook ID, a positive whole number; fractions and zero or negative numbers are rejected")),
	)
)

// RegisterTools registers all repository webhook tools on the MCP server.
func RegisterTools(s *server.MCPServer, f forge.Factory) {
	h := handlers{forge: f}
	s.AddTool(ListRepoHooksTool, h.ListRepoHooksFn)
	s.AddTool(GetRepoHookTool, h.GetRepoHookFn)
	s.AddTool(CreateRepoHookTool, h.CreateRepoHookFn)
	s.AddTool(EditRepoHookTool, h.EditRepoHookFn)
	s.AddTool(DeleteRepoHookTool, h.DeleteRepoHookFn)
	s.AddTool(TestRepoHookTool, h.TestRepoHookFn)
}

// handlers serve the webhook tools and resources. They hold the factory rather
// than a port so each call is bound to the credential of its own request. Each
// tool handler is the MCP adapter of one use case in internal/app/hook: decode the
// arguments, run the use case, render the result.
type handlers struct{ forge forge.Factory }

// hookPayload is the safe form of a webhook, shared by the tools and the
// resources. Config.secret is never included (see model.SafeHook).
type hookPayload = model.HookPayload

// Argument structs, one per tool. `json` names the MCP argument and
// `required:"true"` marks the required ones; mcptest.ArgsMatchSchema keeps
// them in agreement with the tool definitions. An optional flag that must tell
// "not passed" from false is a pointer.

type listArgs struct {
	Owner string  `json:"owner" required:"true"`
	Repo  string  `json:"repo" required:"true"`
	Page  float64 `json:"page" required:"true"`
	Limit float64 `json:"limit" required:"true"`
}

type hookRefArgs struct {
	Owner string  `json:"owner" required:"true"`
	Repo  string  `json:"repo" required:"true"`
	ID    float64 `json:"id" required:"true"`
}

type createArgs struct {
	Owner        string `json:"owner" required:"true"`
	Repo         string `json:"repo" required:"true"`
	URL          string `json:"url" required:"true"`
	Type         string `json:"type"`
	ContentType  string `json:"content_type"`
	Secret       string `json:"secret"`
	HTTPMethod   string `json:"http_method"`
	BranchFilter string `json:"branch_filter"`
	Events       string `json:"events"`
	Active       *bool  `json:"active"`
}

// The string arguments of an edit are decode.Arg, not string: an argument the
// caller sent empty must read as sent, so it can clear the stored value, and
// an absent one must read as left alone.
type editArgs struct {
	Owner        string     `json:"owner" required:"true"`
	Repo         string     `json:"repo" required:"true"`
	ID           float64    `json:"id" required:"true"`
	URL          decode.Arg `json:"url" schema:"string"`
	ContentType  decode.Arg `json:"content_type" schema:"string"`
	Secret       decode.Arg `json:"secret" schema:"string"`
	HTTPMethod   decode.Arg `json:"http_method" schema:"string"`
	BranchFilter decode.Arg `json:"branch_filter" schema:"string"`
	Events       decode.Arg `json:"events" schema:"string"`
	Active       *bool      `json:"active"`
}

// optString is the string an argument carries when the caller sent a string,
// empty included, and nil otherwise. Absent, null and wrongly typed arguments
// are all nil, as the hand-written handler read them.
func optString(a decode.Arg, name string) *string {
	s, ok, err := a.String(name)
	if !ok || err != nil {
		return nil
	}
	return &s
}

// run is the shared shape of a tool call: decode the arguments into A, take the
// use cases bound to this request's credential, and hand both to do, which
// runs one use case and renders its result.
func run[A any](ctx context.Context, h handlers, req mcp.CallToolRequest, name string,
	do func(context.Context, *apphook.Service, A) (*mcp.CallToolResult, error),
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
	return do(ctx, apphook.New(hookadapter.New(port)), args)
}

func (h handlers) ListRepoHooksFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "ListRepoHooksFn", func(ctx context.Context, s *apphook.Service, a listArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render(s.List(ctx, apphook.ListRequest{Owner: a.Owner, Repo: a.Repo, Page: a.Page, Limit: a.Limit}))
	})
}

func (h handlers) GetRepoHookFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "GetRepoHookFn", func(ctx context.Context, s *apphook.Service, a hookRefArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render(s.Get(ctx, apphook.HookRef{Owner: a.Owner, Repo: a.Repo, ID: a.ID}))
	})
}

func (h handlers) CreateRepoHookFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "CreateRepoHookFn", func(ctx context.Context, s *apphook.Service, a createArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render(s.Create(ctx, apphook.CreateRequest{
			Owner: a.Owner, Repo: a.Repo, URL: a.URL, Type: a.Type, ContentType: a.ContentType, Secret: a.Secret,
			HTTPMethod: a.HTTPMethod, BranchFilter: a.BranchFilter, Events: a.Events, Active: a.Active,
		}))
	})
}

func (h handlers) EditRepoHookFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "EditRepoHookFn", func(ctx context.Context, s *apphook.Service, a editArgs) (*mcp.CallToolResult, error) {
		return mcpresult.Render(s.Edit(ctx, apphook.EditRequest{
			HookRef: apphook.HookRef{Owner: a.Owner, Repo: a.Repo, ID: a.ID},
			URL:     optString(a.URL, "url"), ContentType: optString(a.ContentType, "content_type"),
			Secret: optString(a.Secret, "secret"), HTTPMethod: optString(a.HTTPMethod, "http_method"),
			BranchFilter: optString(a.BranchFilter, "branch_filter"), Events: optString(a.Events, "events"),
			Active: a.Active,
		}))
	})
}

func (h handlers) DeleteRepoHookFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "DeleteRepoHookFn", func(ctx context.Context, s *apphook.Service, a hookRefArgs) (*mcp.CallToolResult, error) {
		if err := s.Delete(ctx, apphook.HookRef{Owner: a.Owner, Repo: a.Repo, ID: a.ID}); err != nil {
			return mcpresult.Fail(err)
		}
		return to.TextResult(fmt.Sprintf("Deleted webhook %d on %s/%s", int64(a.ID), a.Owner, a.Repo))
	})
}

func (h handlers) TestRepoHookFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return run(ctx, h, req, "TestRepoHookFn", func(ctx context.Context, s *apphook.Service, a hookRefArgs) (*mcp.CallToolResult, error) {
		if err := s.Test(ctx, apphook.HookRef{Owner: a.Owner, Repo: a.Repo, ID: a.ID}); err != nil {
			return mcpresult.Fail(err)
		}
		return to.TextResult(map[string]bool{"triggered": true})
	})
}
