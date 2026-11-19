// SPDX-License-Identifier: GPL-3.0-or-later

package user

import (
	"context"
	"errors"
	"time"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/list"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/model"
	appuser "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/user"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/annotate"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/decode"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/mcpresult"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/params"

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
		annotate.Read(),
		mcp.WithDescription("Check and list user notifications (page/limit). Returns a response envelope {notifications, page, limit, count, has_next}. has_next true means a further page may exist; re-issue the call with page incremented to fetch it."),
		mcp.WithBoolean("all", mcp.Description("Include read notifications (default: false)")),
		mcp.WithString("since", mcp.Description(params.Since)),
		mcp.WithString("before", mcp.Description(params.Before)),
		mcp.WithNumber("page", mcp.Description(params.Page), mcp.DefaultNumber(1)),
		mcp.WithNumber("limit", mcp.Description(params.Limit), mcp.DefaultNumber(20)),
	)

	GetNotificationThreadTool = mcp.NewTool(
		GetNotificationThreadToolName,
		annotate.Read(),
		mcp.WithDescription("Get detailed info on a single notification thread"),
		mcp.WithNumber("id", mcp.Description("Notification ID"), mcp.Required()),
	)

	MarkNotificationReadTool = mcp.NewTool(
		MarkNotificationReadToolName,
		annotate.Write(),
		mcp.WithDescription("Mark a single notification thread as read"),
		mcp.WithNumber("id", mcp.Description("Notification ID"), mcp.Required()),
	)

	MarkAllNotificationsReadTool = mcp.NewTool(
		MarkAllNotificationsReadToolName,
		annotate.Write(),
		mcp.WithDescription("Acknowledge all notifications"),
		mcp.WithString("last_read_at", mcp.Description("Optional RFC3339 time")),
	)

	ListRepoNotificationsTool = mcp.NewTool(
		ListRepoNotificationsToolName,
		annotate.Read(),
		mcp.WithDescription("Filter notifications scoped to a single repository (page/limit). Returns a response envelope {notifications, page, limit, count, has_next}. has_next true means a further page may exist; re-issue the call with page incremented to fetch it."),
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
		annotate.Write(),
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
// Each struct's Validate runs before the credential lookup.

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
	ID *float64 `json:"id" required:"true" numeric:"string"`
}

type markAllArgs struct {
	LastReadAt string `json:"last_read_at"`
}

type repoMarkArgs struct {
	Owner      *string `json:"owner" required:"true"`
	Repo       *string `json:"repo" required:"true"`
	LastReadAt string  `json:"last_read_at"`
}

// Validate checks the paging arguments, as every list tool does.
func (a checkArgs) Validate() error {
	_, _, err := paging(a.Page, a.Limit)
	return err
}

func (a checkArgs) input() (appuser.NotificationsInput, error) {
	return notificationsInput(a.All, a.Since, a.Before, a.Page, a.Limit)
}

// Validate checks owner and repo, then the paging arguments.
func (a repoCheckArgs) Validate() error {
	if a.Owner == nil || a.Repo == nil {
		return errOwnerRepo
	}
	_, _, err := paging(a.Page, a.Limit)
	return err
}

func (a repoCheckArgs) input() (appuser.NotificationsInput, error) {
	return notificationsInput(a.All, a.Since, a.Before, a.Page, a.Limit)
}

// Validate checks that the thread id is present.
func (a threadArgs) Validate() error {
	if a.ID == nil {
		return errIDRequired
	}
	return nil
}

// Validate checks owner and repo.
func (a repoMarkArgs) Validate() error {
	if a.Owner == nil || a.Repo == nil {
		return errOwnerRepo
	}
	return nil
}

// paging reads the page and limit arguments through the numeric-argument
// module, so a bad value is refused with the same message as on every list tool.
func paging(page, limit float64) (int, int, error) {
	return decode.PagingArgs(page, limit)
}

// notificationPage is one page of notifications in the wire form of the list tools.
func notificationPage(b list.Bounded[*model.NotificationThread]) model.NotificationPage {
	return model.NotificationPage{Notifications: b.Items, PageMeta: model.PageMetaOf(b)}
}

func notificationsInput(all bool, since, before string, pageArg, limitArg float64) (appuser.NotificationsInput, error) {
	page, limit, err := paging(pageArg, limitArg)
	return appuser.NotificationsInput{All: all, Since: since, Before: before, Page: page, Limit: limit}, err
}

// badRequest sends an input error the pipeline did not catch as a tool error.
// Validate runs first, so this is unreachable for a valid request.
func badRequest(err error) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError(err.Error()), nil
}

func checkNotifications(ctx context.Context, svc *appuser.Service, a checkArgs) (*mcp.CallToolResult, error) {
	started := time.Now()
	in, err := a.input()
	if err != nil {
		return badRequest(err)
	}
	page, err := svc.Notifications(ctx, in)
	logAPICall(ctx, "GET", "/notifications", started, err)
	return mcpresult.Render(notificationPage(page), err)
}

func getNotificationThread(ctx context.Context, svc *appuser.Service, a threadArgs) (*mcp.CallToolResult, error) {
	started := time.Now()
	id := int64(*a.ID)
	thread, err := svc.Thread(ctx, id)
	logAPICall(ctx, "GET", apiEndpoint("notifications", "threads", id), started, err)
	return mcpresult.Render(thread, err)
}

func markNotificationRead(ctx context.Context, svc *appuser.Service, a threadArgs) (*mcp.CallToolResult, error) {
	started := time.Now()
	id := int64(*a.ID)
	thread, err := svc.MarkRead(ctx, id)
	logAPICall(ctx, "PATCH", apiEndpoint("notifications", "threads", id), started, err)
	return mcpresult.Render(thread, err)
}

func markAllNotificationsRead(ctx context.Context, svc *appuser.Service, a markAllArgs) (*mcp.CallToolResult, error) {
	started := time.Now()
	err := svc.MarkAllRead(ctx, a.LastReadAt)
	logAPICall(ctx, "PUT", "/notifications", started, err)
	return mcpresult.Render("All notifications marked as read", err)
}

func listRepoNotifications(ctx context.Context, svc *appuser.Service, a repoCheckArgs) (*mcp.CallToolResult, error) {
	started := time.Now()
	owner, repo := *a.Owner, *a.Repo
	in, err := a.input()
	if err != nil {
		return badRequest(err)
	}
	page, err := svc.RepoNotifications(ctx, appuser.RepoNotificationsInput{
		Owner: owner, Repo: repo, NotificationsInput: in,
	})
	logAPICall(ctx, "GET", apiEndpoint("repos", owner, repo, "notifications"), started, err)
	return mcpresult.Render(notificationPage(page), err)
}

func markRepoNotificationsRead(ctx context.Context, svc *appuser.Service, a repoMarkArgs) (*mcp.CallToolResult, error) {
	started := time.Now()
	owner, repo := *a.Owner, *a.Repo
	err := svc.MarkRepoRead(ctx, owner, repo, a.LastReadAt)
	logAPICall(ctx, "PUT", apiEndpoint("repos", owner, repo, "notifications"), started, err)
	return mcpresult.Render("Repo notifications marked as read", err)
}

var (
	errIDRequired = errors.New("id is required and must be a number")
	errOwnerRepo  = errors.New("owner and repo are required")
)
