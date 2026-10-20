// SPDX-License-Identifier: GPL-3.0-or-later

package org

import (
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/operation/params"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	ListOrgTeamsToolName     = "list_org_teams"
	CreateOrgTeamToolName    = "create_org_team"
	AddTeamMemberToolName    = "add_team_member"
	RemoveTeamMemberToolName = "remove_team_member"
	AddTeamRepoToolName      = "add_team_repo"
	RemoveTeamRepoToolName   = "remove_team_repo"
)

var (
	ListOrgTeamsTool = mcp.NewTool(
		ListOrgTeamsToolName,
		mcp.WithDescription("List teams in an organization. Returns a response envelope {teams, page, limit, count, has_next}. has_next true means a further page may exist; re-issue the call with page incremented to fetch it."),
		mcp.WithString("org", mcp.Required(), mcp.Description(params.Org)),
		mcp.WithNumber("page", mcp.Required(), mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("limit", mcp.Required(), mcp.Description(params.Limit), mcp.DefaultNumber(100), mcp.Min(1)),
	)

	CreateOrgTeamTool = mcp.NewTool(
		CreateOrgTeamToolName,
		mcp.WithDescription("Create a team in an organization"),
		mcp.WithString("org", mcp.Required(), mcp.Description(params.Org)),
		mcp.WithString("name", mcp.Required(), mcp.Description("Team name")),
		mcp.WithString("description", mcp.Description(params.Description)),
		mcp.WithString("permission", mcp.Description("Access level: read, write, or admin (default: read)")),
		mcp.WithBoolean("can_create_org_repo", mcp.Description("Whether members can create repos in the org")),
		mcp.WithBoolean("includes_all_repositories", mcp.Description("Whether team has access to all org repos")),
	)

	AddTeamMemberTool = mcp.NewTool(
		AddTeamMemberToolName,
		mcp.WithDescription("Add a user to a team"),
		mcp.WithNumber("team_id", mcp.Required(), mcp.Description("Team ID")),
		mcp.WithString("user", mcp.Required(), mcp.Description(params.User)),
	)

	RemoveTeamMemberTool = mcp.NewTool(
		RemoveTeamMemberToolName,
		mcp.WithDescription("Remove a user from a team"),
		mcp.WithNumber("team_id", mcp.Required(), mcp.Description("Team ID")),
		mcp.WithString("user", mcp.Required(), mcp.Description(params.User)),
	)

	AddTeamRepoTool = mcp.NewTool(
		AddTeamRepoToolName,
		mcp.WithDescription("Add a repository to a team"),
		mcp.WithNumber("team_id", mcp.Required(), mcp.Description("Team ID")),
		mcp.WithString("org", mcp.Required(), mcp.Description(params.Org)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
	)

	RemoveTeamRepoTool = mcp.NewTool(
		RemoveTeamRepoToolName,
		mcp.WithDescription("Remove a repository from a team"),
		mcp.WithNumber("team_id", mcp.Required(), mcp.Description("Team ID")),
		mcp.WithString("org", mcp.Required(), mcp.Description(params.Org)),
		mcp.WithString("repo", mcp.Required(), mcp.Description(params.Repo)),
	)
)
