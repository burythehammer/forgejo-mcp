// SPDX-License-Identifier: GPL-3.0-or-later

package org

import (
	"context"
	"errors"
	"fmt"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/params"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/log"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/to"

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
		mcp.WithDescription("List my organizations"),
		mcp.WithNumber("page", mcp.Required(), mcp.Description(params.Page), mcp.DefaultNumber(1), mcp.Min(1)),
		mcp.WithNumber("limit", mcp.Required(), mcp.Description(params.Limit), mcp.DefaultNumber(100), mcp.Min(1)),
	)

	ListUserOrgsTool = mcp.NewTool(
		ListUserOrgsToolName,
		mcp.WithDescription("List a user's organizations"),
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

// handlers serve the org tools from the Forgejo port. The port is resolved per
// request, so each request is served under its own credential.
type handlers struct {
	forge forge.Factory
}

func RegisterTool(s *server.MCPServer, f forge.Factory) {
	h := handlers{forge: f}
	s.AddTool(CreateOrgTool, h.CreateOrgFn)
	s.AddTool(GetOrgTool, h.GetOrgFn)
	s.AddTool(ListMyOrgsTool, h.ListMyOrgsFn)
	s.AddTool(ListUserOrgsTool, h.ListUserOrgsFn)
	s.AddTool(EditOrgTool, h.EditOrgFn)
	s.AddTool(DeleteOrgTool, h.DeleteOrgFn)

	// Membership
	s.AddTool(ListOrgMembersTool, h.ListOrgMembersFn)
	s.AddTool(CheckOrgMembershipTool, h.CheckOrgMembershipFn)
	s.AddTool(RemoveOrgMemberTool, h.RemoveOrgMemberFn)

	// Teams
	s.AddTool(ListOrgTeamsTool, h.ListOrgTeamsFn)
	s.AddTool(CreateOrgTeamTool, h.CreateOrgTeamFn)
	s.AddTool(AddTeamMemberTool, h.AddTeamMemberFn)
	s.AddTool(RemoveTeamMemberTool, h.RemoveTeamMemberFn)
	s.AddTool(AddTeamRepoTool, h.AddTeamRepoFn)
	s.AddTool(RemoveTeamRepoTool, h.RemoveTeamRepoFn)
}

func (h handlers) CreateOrgFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called CreateOrgFn")
	name, ok := req.GetArguments()["name"].(string)
	if !ok {
		return to.ErrorResult(errors.New("organization name is required"))
	}

	opt := forgejo_sdk.CreateOrgOption{
		Name: name,
	}
	if v, ok := req.GetArguments()["full_name"].(string); ok {
		opt.FullName = v
	}
	if v, ok := req.GetArguments()["description"].(string); ok {
		opt.Description = v
	}
	if v, ok := req.GetArguments()["website"].(string); ok {
		opt.Website = v
	}
	if v, ok := req.GetArguments()["location"].(string); ok {
		opt.Location = v
	}
	if v, ok := req.GetArguments()["visibility"].(string); ok {
		opt.Visibility = forgejo_sdk.VisibleType(v)
	}

	fg, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	org, err := fg.CreateOrg(ctx, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("create org err: %w", err))
	}
	return to.TextResult(org)
}

func (h handlers) GetOrgFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called GetOrgFn")
	orgName, ok := req.GetArguments()["org"].(string)
	if !ok {
		return to.ErrorResult(errors.New("organization name is required"))
	}

	fg, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	org, err := fg.GetOrg(ctx, orgName)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get org err: %w", err))
	}
	return to.TextResult(org)
}

func (h handlers) ListMyOrgsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListMyOrgsFn")
	page, ok := req.GetArguments()["page"].(float64)
	if !ok {
		page = 1
	}
	limit, ok := req.GetArguments()["limit"].(float64)
	if !ok {
		limit = 100
	}

	opt := forgejo_sdk.ListOrgsOptions{
		ListOptions: forgejo_sdk.ListOptions{
			Page:     int(page),
			PageSize: int(limit),
		},
	}
	fg, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	orgs, err := fg.ListMyOrgs(ctx, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list my orgs err: %w", err))
	}
	return to.TextResult(orgs)
}

func (h handlers) ListUserOrgsFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called ListUserOrgsFn")
	user, ok := req.GetArguments()["user"].(string)
	if !ok {
		return to.ErrorResult(errors.New("username is required"))
	}
	page, ok := req.GetArguments()["page"].(float64)
	if !ok {
		page = 1
	}
	limit, ok := req.GetArguments()["limit"].(float64)
	if !ok {
		limit = 100
	}

	opt := forgejo_sdk.ListOrgsOptions{
		ListOptions: forgejo_sdk.ListOptions{
			Page:     int(page),
			PageSize: int(limit),
		},
	}
	fg, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	orgs, err := fg.ListUserOrgs(ctx, user, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("list user orgs err: %w", err))
	}
	return to.TextResult(orgs)
}

func (h handlers) EditOrgFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called EditOrgFn")
	orgName, ok := req.GetArguments()["org"].(string)
	if !ok {
		return to.ErrorResult(errors.New("organization name is required"))
	}

	opt := forgejo_sdk.EditOrgOption{}
	if v, ok := req.GetArguments()["full_name"].(string); ok {
		opt.FullName = v
	}
	if v, ok := req.GetArguments()["description"].(string); ok {
		opt.Description = v
	}
	if v, ok := req.GetArguments()["website"].(string); ok {
		opt.Website = v
	}
	if v, ok := req.GetArguments()["location"].(string); ok {
		opt.Location = v
	}
	if v, ok := req.GetArguments()["visibility"].(string); ok {
		opt.Visibility = forgejo_sdk.VisibleType(v)
	}

	fg, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	err = fg.EditOrg(ctx, orgName, opt)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("edit org err: %w", err))
	}

	// Fetch updated org to return
	org, err := fg.GetOrg(ctx, orgName)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("get updated org err: %w", err))
	}
	return to.TextResult(org)
}

func (h handlers) DeleteOrgFn(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Debugf("Called DeleteOrgFn")
	orgName, ok := req.GetArguments()["org"].(string)
	if !ok {
		return to.ErrorResult(errors.New("organization name is required"))
	}

	fg, err := h.forge.For(ctx)
	if err != nil {
		return to.ErrorResult(err)
	}
	err = fg.DeleteOrg(ctx, orgName)
	if err != nil {
		return to.ErrorResult(fmt.Errorf("delete org err: %w", err))
	}
	return to.TextResult(fmt.Sprintf("Organization '%s' deleted successfully", orgName))
}
