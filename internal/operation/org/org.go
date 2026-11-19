// SPDX-License-Identifier: GPL-3.0-or-later

package org

import (
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/annotate"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/params"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/pipeline"

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
		annotate.Write(),
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
		annotate.Read(),
		mcp.WithDescription("Get organization details"),
		mcp.WithString("org", mcp.Required(), mcp.Description(params.Org)),
	)

	ListMyOrgsTool = mcp.NewTool(
		ListMyOrgsToolName,
		annotate.Read(),
		mcp.WithDescription("List my organizations. Returns a response envelope {organizations, page, limit, count, has_next}. has_next true means a further page may exist; re-issue the call with page incremented to fetch it."),
		mcp.WithNumber("page", mcp.Required(), mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("limit", mcp.Required(), mcp.Description(params.Limit), mcp.DefaultNumber(100), mcp.Min(1)),
	)

	ListUserOrgsTool = mcp.NewTool(
		ListUserOrgsToolName,
		annotate.Read(),
		mcp.WithDescription("List a user's organizations. Returns a response envelope {organizations, page, limit, count, has_next}. has_next true means a further page may exist; re-issue the call with page incremented to fetch it."),
		mcp.WithString("user", mcp.Required(), mcp.Description(params.User)),
		mcp.WithNumber("page", mcp.Required(), mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("limit", mcp.Required(), mcp.Description(params.Limit), mcp.DefaultNumber(100), mcp.Min(1)),
	)

	EditOrgTool = mcp.NewTool(
		EditOrgToolName,
		annotate.Write(),
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
		annotate.Destructive(),
		mcp.WithDescription("Delete an organization. WARNING: This is destructive and irreversible — all repos, teams, and data will be permanently removed"),
		mcp.WithString("org", mcp.Required(), mcp.Description(params.Org)),
	)
)

// RegisterTool registers the organization, membership and team tools. Each
// call takes its Forgejo port from f, so a request is served with the
// credential that request carries.
func RegisterTool(s *server.MCPServer, f forge.Factory) {
	h := handlers{forge: f}
	s.AddTool(CreateOrgTool, pipeline.Handle(h.service, createOrg))
	s.AddTool(GetOrgTool, pipeline.Handle(h.service, getOrg))
	s.AddTool(ListMyOrgsTool, pipeline.Handle(h.service, listMyOrgs))
	s.AddTool(ListUserOrgsTool, pipeline.Handle(h.service, listUserOrgs))
	s.AddTool(EditOrgTool, pipeline.Handle(h.service, editOrg))
	s.AddTool(DeleteOrgTool, pipeline.Handle(h.service, deleteOrg))

	s.AddTool(ListOrgMembersTool, pipeline.Handle(h.service, listOrgMembers))
	s.AddTool(CheckOrgMembershipTool, pipeline.Handle(h.service, checkOrgMembership))
	s.AddTool(RemoveOrgMemberTool, pipeline.Handle(h.service, removeOrgMember))

	s.AddTool(ListOrgTeamsTool, pipeline.Handle(h.service, listOrgTeams))
	s.AddTool(CreateOrgTeamTool, pipeline.Handle(h.service, createOrgTeam))
	s.AddTool(AddTeamMemberTool, pipeline.Handle(h.service, addTeamMember))
	s.AddTool(RemoveTeamMemberTool, pipeline.Handle(h.service, removeTeamMember))
	s.AddTool(AddTeamRepoTool, pipeline.Handle(h.service, addTeamRepo))
	s.AddTool(RemoveTeamRepoTool, pipeline.Handle(h.service, removeTeamRepo))
}
