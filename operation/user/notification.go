// SPDX-License-Identifier: GPL-3.0-or-later

package user

import (
	"context"
	"errors"
	"fmt"
	"time"

	appuser "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/user"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/decode"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/params"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/to"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	CheckNotificationsToolName        = "check_notifications"
	GetNotificationThreadToolName     = "get_notification_thread"
	MarkNotificationReadToolName      = "mark_notification_read"
	MarkAllNotificationsReadToolName  = "mark_all_notifications_read"
	ListRepoNotificationsToolName     = "list_repo_notifications"
	MarkRepoNotificationsReadToolName = "mark_repo_notifications_read"
)

var (
	CheckNotificationsTool = mcp.NewTool(
		CheckNotificationsToolName,
		mcp.WithDescription("Check and list user notifications"),
		mcp.WithBoolean("all", mcp.Description("Include read notifications (default: false)")),
		mcp.WithString("since", mcp.Description(params.Since)),
		mcp.WithString("before", mcp.Description(params.Before)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(20)),
	)

	GetNotificationThreadTool = mcp.NewTool(
		GetNotificationThreadToolName,
		mcp.WithDescription("Get detailed info on a single notification thread"),
		mcp.WithNumber("id", mcp.Description("Notification ID"), mcp.Required()),
	)

	MarkNotificationReadTool = mcp.NewTool(
		MarkNotificationReadToolName,
		mcp.WithDescription("Mark a single notification thread as read"),
		mcp.WithNumber("id", mcp.Description("Notification ID"), mcp.Required()),
	)

	MarkAllNotificationsReadTool = mcp.NewTool(
		MarkAllNotificationsReadToolName,
		mcp.WithDescription("Acknowledge all notifications"),
		mcp.WithString("last_read_at", mcp.Description("Optional RFC3339 time")),
	)

	ListRepoNotificationsTool = mcp.NewTool(
		ListRepoNotificationsToolName,
		mcp.WithDescription("Filter notifications scoped to a single repository"),
		mcp.WithString("owner", mcp.Description("Repository owner"), mcp.Required()),
		mcp.WithString("repo", mcp.Description("Repository name"), mcp.Required()),
		mcp.WithBoolean("all", mcp.Description("Include read notifications (default: false)")),
		mcp.WithString("since", mcp.Description(params.Since)),
		mcp.WithString("before", mcp.Description(params.Before)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(20)),
	)

	MarkRepoNotificationsReadTool = mcp.NewTool(
		MarkRepoNotificationsReadToolName,
		mcp.WithDescription("Mark all notifications in a specific repo as read"),
		mcp.WithString("owner", mcp.Description("Repository owner"), mcp.Required()),
		mcp.WithString("repo", mcp.Description("Repository name"), mcp.Required()),
		mcp.WithString("last_read_at", mcp.Description("Optional RFC3339 time")),
	)
)

// Argument structs, one per tool. `json` names the MCP argument and
// `required:"true"` marks the required ones; mcptest.ArgsMatchSchema keeps
// them in agreement with the tool definitions. Numbers are float64 so the
// forgiving decoder truncates them as the original handlers did. An id, owner
// or repo is a pointer so that a missing or wrongly typed value reads as absent.

type checkArgs struct {
	All    bool    `json:"all"`
	Since  string  `json:"since"`
	Before string  `json:"before"`
	Page   float64 `json:"page"`
	Limit  float64 `json:"limit"`
}

type repoCheckArgs struct {
	Owner  *string `json:"owner" required:"true"`
	Repo   *string `json:"repo" required:"true"`
	All    bool    `json:"all"`
	Since  string  `json:"since"`
	Before string  `json:"before"`
	Page   float64 `json:"page"`
	Limit  float64 `json:"limit"`
}

type threadArgs struct {
	ID *float64 `json:"id" required:"true"`
}

type markAllArgs struct {
	LastReadAt string `json:"last_read_at"`
}

type repoMarkArgs struct {
	Owner      *string `json:"owner" required:"true"`
	Repo       *string `json:"repo" required:"true"`
	LastReadAt string  `json:"last_read_at"`
}

// paging applies the tools' defaults to the page and limit arguments.
func paging(page, limit float64) (int, int) {
	if page == 0 {
		page = 1
	}
	if limit == 0 {
		limit = 20
	}
	return int(page), int(limit)
}

// start begins a tool call: it attaches the tool's name to the context, logs
// the call and decodes the arguments. A decode failure is returned as an error.
func start[A any](ctx context.Context, req mcp.CallToolRequest, name string) (context.Context, time.Time, A, error) {
	ctx, _ = log.WithMCPContext(ctx, name)
	started := time.Now()
	log.LogMCPToolStart(ctx, name, req.GetArguments())
	var args A
	err := decode.Lax(req, &args)
	return ctx, started, args, err
}

// refuse logs and renders a request refused before Forgejo was called.
func refuse(ctx context.Context, name string, started time.Time, err error) (*mcp.CallToolResult, error) {
	log.LogMCPToolError(ctx, name, time.Since(started), err)
	return to.ErrorResult(err)
}

// finish logs and renders the outcome of a call that went through the use
// cases: the result as JSON, or the error.
func finish(ctx context.Context, name string, started time.Time, summary string, v any, err error) (*mcp.CallToolResult, error) {
	if err != nil {
		log.LogMCPToolError(ctx, name, time.Since(started), err)
		return to.ErrorResult(err)
	}
	log.LogMCPToolComplete(ctx, name, time.Since(started), summary)
	return to.TextResult(v)
}

func notificationsInput(all bool, since, before string, pageArg, limitArg float64) appuser.NotificationsInput {
	page, limit := paging(pageArg, limitArg)
	return appuser.NotificationsInput{All: all, Since: since, Before: before, Page: page, Limit: limit}
}

func (h handlers) CheckNotificationsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ctx, started, a, err := start[checkArgs](ctx, req, CheckNotificationsToolName)
	if err != nil {
		return refuse(ctx, CheckNotificationsToolName, started, err)
	}
	threads, err := h.service().Notifications(ctx, notificationsInput(a.All, a.Since, a.Before, a.Page, a.Limit))
	logAPICall(ctx, "GET", "/notifications", started, err)
	return finish(ctx, CheckNotificationsToolName, started, fmt.Sprintf("Retrieved %d notifications", len(threads)), threads, err)
}

func (h handlers) GetNotificationThreadFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ctx, started, a, err := start[threadArgs](ctx, req, GetNotificationThreadToolName)
	if err != nil {
		return refuse(ctx, GetNotificationThreadToolName, started, err)
	}
	if a.ID == nil {
		return refuse(ctx, GetNotificationThreadToolName, started, errIDRequired)
	}
	id := int64(*a.ID)
	thread, err := h.service().Thread(ctx, id)
	logAPICall(ctx, "GET", forge.APIPath("notifications", "threads", id), started, err)
	return finish(ctx, GetNotificationThreadToolName, started, "Retrieved notification thread", thread, err)
}

func (h handlers) MarkNotificationReadFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ctx, started, a, err := start[threadArgs](ctx, req, MarkNotificationReadToolName)
	if err != nil {
		return refuse(ctx, MarkNotificationReadToolName, started, err)
	}
	if a.ID == nil {
		return refuse(ctx, MarkNotificationReadToolName, started, errIDRequired)
	}
	id := int64(*a.ID)
	thread, err := h.service().MarkRead(ctx, id)
	logAPICall(ctx, "PATCH", forge.APIPath("notifications", "threads", id), started, err)
	return finish(ctx, MarkNotificationReadToolName, started, "Marked notification as read", thread, err)
}

func (h handlers) MarkAllNotificationsReadFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ctx, started, a, err := start[markAllArgs](ctx, req, MarkAllNotificationsReadToolName)
	if err != nil {
		return refuse(ctx, MarkAllNotificationsReadToolName, started, err)
	}
	err = h.service().MarkAllRead(ctx, a.LastReadAt)
	logAPICall(ctx, "PUT", "/notifications", started, err)
	if err != nil {
		return finish(ctx, MarkAllNotificationsReadToolName, started, "", nil, err)
	}
	return finish(ctx, MarkAllNotificationsReadToolName, started, "Marked all notifications as read", "All notifications marked as read", nil)
}

func (h handlers) ListRepoNotificationsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ctx, started, a, err := start[repoCheckArgs](ctx, req, ListRepoNotificationsToolName)
	if err != nil {
		return refuse(ctx, ListRepoNotificationsToolName, started, err)
	}
	if a.Owner == nil || a.Repo == nil {
		return refuse(ctx, ListRepoNotificationsToolName, started, errOwnerRepo)
	}
	owner, repo := *a.Owner, *a.Repo
	threads, err := h.service().RepoNotifications(ctx, appuser.RepoNotificationsInput{
		Owner: owner, Repo: repo, NotificationsInput: notificationsInput(a.All, a.Since, a.Before, a.Page, a.Limit),
	})
	logAPICall(ctx, "GET", forge.APIPath("repos", owner, repo, "notifications"), started, err)
	return finish(ctx, ListRepoNotificationsToolName, started,
		fmt.Sprintf("Retrieved %d notifications for repo %s/%s", len(threads), owner, repo), threads, err)
}

func (h handlers) MarkRepoNotificationsReadFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ctx, started, a, err := start[repoMarkArgs](ctx, req, MarkRepoNotificationsReadToolName)
	if err != nil {
		return refuse(ctx, MarkRepoNotificationsReadToolName, started, err)
	}
	if a.Owner == nil || a.Repo == nil {
		return refuse(ctx, MarkRepoNotificationsReadToolName, started, errOwnerRepo)
	}
	owner, repo := *a.Owner, *a.Repo
	err = h.service().MarkRepoRead(ctx, owner, repo, a.LastReadAt)
	logAPICall(ctx, "PUT", forge.APIPath("repos", owner, repo, "notifications"), started, err)
	if err != nil {
		return finish(ctx, MarkRepoNotificationsReadToolName, started, "", nil, err)
	}
	return finish(ctx, MarkRepoNotificationsReadToolName, started,
		fmt.Sprintf("Marked all notifications for repo %s/%s as read", owner, repo), "Repo notifications marked as read", nil)
}

var (
	errIDRequired = errors.New("id is required and must be a number")
	errOwnerRepo  = errors.New("owner and repo are required")
)
