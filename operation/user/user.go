package user

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
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

// handlers serve the user tools from the Forgejo port. The port is resolved per
// request, so each request is served under its own credential.
type handlers struct {
	forge forge.Factory
}

// apiStatus is the HTTP status to log for an API call: the one Forgejo
// answered a failure with, or 200 for a call that succeeded.
func apiStatus(err error) int {
	if err == nil {
		return http.StatusOK
	}
	return forge.StatusCode(err)
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

	client, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	user, err := client.GetMyUserInfo(ctx)
	duration := time.Since(start)

	statusCode := apiStatus(err)
	forge.LogAPICall(ctx, "GET", "/user", duration, statusCode, err)

	if err != nil {
		log.LogMCPToolError(ctx, GetMyUserInfoToolName, duration, err)
		return to.ErrorResult(fmt.Errorf("get user info err: %w", err))
	}

	log.LogMCPToolComplete(ctx, GetMyUserInfoToolName, duration, fmt.Sprintf("Retrieved info for user: %s", user.UserName))

	return to.TextResult(user)
}
