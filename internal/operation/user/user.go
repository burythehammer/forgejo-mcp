// SPDX-License-Identifier: GPL-3.0-or-later

package user

import (
	"context"
	"net/http"
	"time"

	appuser "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/user"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/useradapter"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/log"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/annotate"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcpresult"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/pipeline"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	GetMyUserInfoToolName = "get_my_user_info"
)

var GetMyUserInfoTool = mcp.NewTool(
	GetMyUserInfoToolName,
	annotate.Read(),
	mcp.WithDescription("Get user info"),
)

// handlers serve the user tools. They hold the factory rather than a port so
// each call is bound to the credential of its own request. Each tool is the
// MCP adapter of one use case in internal/app/user: the request pipeline
// decodes and validates the arguments, and the use case runs with the service
// bound here.
type handlers struct {
	forge forge.Factory
}

// bind is the pipeline Binder for every user tool. The Forgejo port is looked
// up lazily, inside the use case, after the use case has checked its input, so
// an invalid input is refused before any credential lookup, as it always was.
func (h handlers) bind(context.Context) (*appuser.Service, error) {
	return appuser.New(func(ctx context.Context) (appuser.Port, error) {
		port, err := h.forge.For(ctx)
		if err != nil {
			return nil, err
		}
		return useradapter.New(port), nil
	}), nil
}

// handle wraps one user tool in the request pipeline. The tool's name is
// attached to the context first, so each Forgejo API call it makes is logged
// with the request it belongs to.
func handle[A any](name string, h handlers, run func(context.Context, *appuser.Service, A) (*mcp.CallToolResult, error), opts ...pipeline.Option) server.ToolHandlerFunc {
	return pipeline.Handle(h.bind, func(ctx context.Context, svc *appuser.Service, a A) (*mcp.CallToolResult, error) {
		ctx, _ = log.WithMCPContext(ctx, name)
		return run(ctx, svc, a)
	}, opts...)
}

// apiStatus is the HTTP status to log for an API call: the one Forgejo
// answered a failure with, or 200 for a call that succeeded.
func apiStatus(err error) int {
	if err == nil {
		return http.StatusOK
	}
	return forge.StatusCode(err)
}

// logAPICall records an API call that reached Forgejo: one that succeeded, or
// one Forgejo answered with an HTTP status. A request refused before the call
// (bad input, no credential) is not an API call and is not logged as one.
// apiEndpoint is the endpoint label for an API-call log line. The service call
// does the real request; a segment the path builder refuses is logged as a
// placeholder rather than the raw value.
func apiEndpoint(segments ...any) string {
	p, err := forge.APIPath(segments...)
	if err != nil {
		return "(refused path segment)"
	}
	return p
}

func logAPICall(ctx context.Context, method, endpoint string, start time.Time, err error) {
	if err != nil && forge.StatusCode(err) == 0 {
		return
	}
	forge.LogAPICall(ctx, method, endpoint, time.Since(start), apiStatus(err), err)
}

func RegisterTool(s *server.MCPServer, f forge.Factory) {
	h := handlers{forge: f}
	s.AddTool(GetMyUserInfoTool, handle(GetMyUserInfoToolName, h, getMyUserInfo))
	s.AddTool(CheckNotificationsTool, handle(CheckNotificationsToolName, h, checkNotifications))
	s.AddTool(GetNotificationThreadTool, handle(GetNotificationThreadToolName, h, getNotificationThread))
	s.AddTool(MarkNotificationReadTool, handle(MarkNotificationReadToolName, h, markNotificationRead))
	s.AddTool(MarkAllNotificationsReadTool, handle(MarkAllNotificationsReadToolName, h, markAllNotificationsRead))
	s.AddTool(ListRepoNotificationsTool, handle(ListRepoNotificationsToolName, h, listRepoNotifications))
	s.AddTool(MarkRepoNotificationsReadTool, handle(MarkRepoNotificationsReadToolName, h, markRepoNotificationsRead))
}

func getMyUserInfo(ctx context.Context, svc *appuser.Service, _ struct{}) (*mcp.CallToolResult, error) {
	started := time.Now()
	user, err := svc.MyInfo(ctx)
	logAPICall(ctx, "GET", "/user", started, err)
	return mcpresult.Render(user, err)
}
