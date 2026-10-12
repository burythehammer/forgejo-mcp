// SPDX-License-Identifier: GPL-3.0-or-later

package user

import (
	"context"
	"net/http"
	"time"

	appuser "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/user"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/useradapter"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/to"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	GetMyUserInfoToolName = "get_my_user_info"
)

var GetMyUserInfoTool = mcp.NewTool(
	GetMyUserInfoToolName,
	mcp.WithDescription("Get user info"),
)

// handlers serve the user tools. They hold the factory rather than a port so
// each call is bound to the credential of its own request. Each handler is the
// MCP adapter of one use case in app/user: decode the arguments, run the use
// case, render the result and log the call.
type handlers struct {
	forge forge.Factory
}

// service returns the use cases for one request. The port is looked up only
// once a use case has checked its input.
func (h handlers) service() *appuser.Service {
	return appuser.New(func(ctx context.Context) (appuser.Port, error) {
		port, err := h.forge.For(ctx)
		if err != nil {
			return nil, err
		}
		return useradapter.New(port), nil
	})
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
func logAPICall(ctx context.Context, method, endpoint string, start time.Time, err error) {
	if err != nil && forge.StatusCode(err) == 0 {
		return
	}
	forge.LogAPICall(ctx, method, endpoint, time.Since(start), apiStatus(err), err)
}

func RegisterTool(s *server.MCPServer, f forge.Factory) {
	h := handlers{forge: f}
	s.AddTool(GetMyUserInfoTool, h.GetUserInfoFn)
	s.AddTool(CheckNotificationsTool, h.CheckNotificationsFn)
	s.AddTool(GetNotificationThreadTool, h.GetNotificationThreadFn)
	s.AddTool(MarkNotificationReadTool, h.MarkNotificationReadFn)
	s.AddTool(MarkAllNotificationsReadTool, h.MarkAllNotificationsReadFn)
	s.AddTool(ListRepoNotificationsTool, h.ListRepoNotificationsFn)
	s.AddTool(MarkRepoNotificationsReadTool, h.MarkRepoNotificationsReadFn)
}

func (h handlers) GetUserInfoFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	ctx, _ = log.WithMCPContext(ctx, GetMyUserInfoToolName)
	start := time.Now()

	log.LogMCPToolStart(ctx, GetMyUserInfoToolName, map[string]interface{}{})

	user, err := h.service().MyInfo(ctx)
	logAPICall(ctx, "GET", "/user", start, err)
	if err != nil {
		log.LogMCPToolError(ctx, GetMyUserInfoToolName, time.Since(start), err)
		return to.ErrorResult(err)
	}

	log.LogMCPToolComplete(ctx, GetMyUserInfoToolName, time.Since(start), "Retrieved info for user: "+user.UserName)

	return to.TextResult(user)
}
