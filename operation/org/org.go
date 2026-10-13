// SPDX-License-Identifier: GPL-3.0-or-later

package org

import (
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/params"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	CreateOrgToolName    = "create_org"
	GetOrgToolName       = "get_org"
	ListMyOrgsToolName   = "list_my_orgs"
	ListUserOrgsToolName = "list_user_orgs"
	EditOrgToolName      = "edit_org"
	DeleteOrgToolName    = "delete_org"
)

var (
	CreateOrgTool = mcp.NewTool(
		CreateOrgToolName,
		mcp.WithDescription("Create an organization"),
		mcp.WithString("name", mcp.Required(), mcp.Description("Organization username")),
		mcp.WithString("full_name", mcp.Description("Display name")),
		mcp.WithString("description", mcp.Description(params.Description)),
		mcp.WithString("website", mcp.Description(params.Website)),
		mcp.WithString("location", mcp.Description("Location")),
		mcp.WithString("visibility", mcp.Description("Visibility: public, limited, or private")),
	)

	GetOrgTool = mcp.NewTool(
		GetOrgToolName,
		mcp.WithDescription("Get organization details"),
		mcp.WithString("org", mcp.Required(), mcp.Description(params.Org)),
	)

	ListMyOrgsTool = mcp.NewTool(
		ListMyOrgsToolName,
		mcp.WithDescription("List my organizations. Returns a response envelope {organizations, page, limit, count, has_next}. has_next true means a further page may exist; re-issue the call with page incremented to fetch it."),
		mcp.WithNumber("page", mcp.Required(), mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("limit", mcp.Required(), mcp.Description(params.Limit), mcp.DefaultNumber(100), mcp.Min(1)),
	)

	ListUserOrgsTool = mcp.NewTool(
		ListUserOrgsToolName,
		mcp.WithDescription("List a user's organizations. Returns a response envelope {organizations, page, limit, count, has_next}. has_next true means a further page may exist; re-issue the call with page incremented to fetch it."),
		mcp.WithString("user", mcp.Required(), mcp.Description(params.User)),
		mcp.WithNumber("page", mcp.Required(), mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("limit", mcp.Required(), mcp.Description(params.Limit), mcp.DefaultNumber(100), mcp.Min(1)),
	)

	EditOrgTool = mcp.NewTool(
		EditOrgToolName,
		mcp.WithDescription("Edit organization settings"),
		mcp.WithString("org", mcp.Required(), mcp.Description(params.Org)),
		mcp.WithString("full_name", mcp.Description("Display name")),
		mcp.WithString("description", mcp.Description(params.Description)),
		mcp.WithString("website", mcp.Description(params.Website)),
		mcp.WithString("location", mcp.Description("Location")),
		mcp.WithString("visibility", mcp.Description("Visibility: public, limited, or private")),
	)

	DeleteOrgTool = mcp.NewTool(
		DeleteOrgToolName,
		mcp.WithDescription("Delete an organization. WARNING: This is destructive and irreversible — all repos, teams, and data will be permanently removed"),
		mcp.WithString("org", mcp.Required(), mcp.Description(params.Org)),
	)
)

// RegisterTool registers the organization, membership and team tools. Each
// call takes its Forgejo port from f, so a request is served with the
// credential that request carries.
func RegisterTool(s *server.MCPServer, f forge.Factory) {
	h := handlers{forge: f}
	s.AddTool(CreateOrgTool, h.CreateOrgFn)
	s.AddTool(GetOrgTool, h.GetOrgFn)
	s.AddTool(ListMyOrgsTool, h.ListMyOrgsFn)
	s.AddTool(ListUserOrgsTool, h.ListUserOrgsFn)
	s.AddTool(EditOrgTool, h.EditOrgFn)
	s.AddTool(DeleteOrgTool, h.DeleteOrgFn)

	s.AddTool(ListOrgMembersTool, h.ListOrgMembersFn)
	s.AddTool(CheckOrgMembershipTool, h.CheckOrgMembershipFn)
	s.AddTool(RemoveOrgMemberTool, h.RemoveOrgMemberFn)

	s.AddTool(ListOrgTeamsTool, h.ListOrgTeamsFn)
	s.AddTool(CreateOrgTeamTool, h.CreateOrgTeamFn)
	s.AddTool(AddTeamMemberTool, h.AddTeamMemberFn)
	s.AddTool(RemoveTeamMemberTool, h.RemoveTeamMemberFn)
	s.AddTool(AddTeamRepoTool, h.AddTeamRepoFn)
	s.AddTool(RemoveTeamRepoTool, h.RemoveTeamRepoFn)
}
