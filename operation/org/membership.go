// SPDX-License-Identifier: GPL-3.0-or-later

package org

import (
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/params"

	"github.com/mark3labs/mcp-go/mcp"
)

const (
	ListOrgMembersToolName     = "list_org_members"
	CheckOrgMembershipToolName = "check_org_membership"
	RemoveOrgMemberToolName    = "remove_org_member"
)

var (
	ListOrgMembersTool = mcp.NewTool(
		ListOrgMembersToolName,
		mcp.WithDescription("List members of an organization"),
		mcp.WithString("org", mcp.Required(), mcp.Description(params.Org)),
		mcp.WithNumber("page", mcp.Required(), mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("limit", mcp.Required(), mcp.Description(params.Limit), mcp.DefaultNumber(100), mcp.Min(1)),
	)

	CheckOrgMembershipTool = mcp.NewTool(
		CheckOrgMembershipToolName,
		mcp.WithDescription("Check if a user is a member of an organization"),
		mcp.WithString("org", mcp.Required(), mcp.Description(params.Org)),
		mcp.WithString("user", mcp.Required(), mcp.Description(params.User)),
	)

	RemoveOrgMemberTool = mcp.NewTool(
		RemoveOrgMemberToolName,
		mcp.WithDescription("Remove a member from an organization"),
		mcp.WithString("org", mcp.Required(), mcp.Description(params.Org)),
		mcp.WithString("user", mcp.Required(), mcp.Description(params.User)),
	)
)
