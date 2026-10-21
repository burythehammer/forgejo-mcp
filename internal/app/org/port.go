// SPDX-License-Identifier: GPL-3.0-or-later

// Package org holds the organization, membership and team use cases behind the
// org tools. Like all of internal/app/ it is MCP-free and SDK-free
// (the depguard rules in .golangci.yml): use cases take typed inputs, reach
// Forgejo only through Port, and return repo-owned types from internal/app/model for an
// MCP adapter in internal/operation/org to render.
package org

import (
	"context"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/list"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
)

// Port is the organization domain as the use cases need it, in repo-owned
// types. internal/forge/orgadapter implements it over forge.Orgs.
type Port interface {
	CreateOrg(ctx context.Context, in CreateOrgInput) (*model.Organization, error)
	GetOrg(ctx context.Context, org string) (*model.Organization, error)
	EditOrg(ctx context.Context, org string, in EditOrgInput) error
	DeleteOrg(ctx context.Context, org string) error
	// MaxResponseItems is the instance's pagination ceiling; ok is false when it
	// cannot be determined, which is "unknown", never a ceiling of zero.
	MaxResponseItems(ctx context.Context) (limit int, ok bool)
	ListMyOrgs(ctx context.Context, p Paging) ([]*model.Organization, error)
	ListUserOrgs(ctx context.Context, user string, p Paging) ([]*model.Organization, error)

	ListOrgMembers(ctx context.Context, org string, p Paging) ([]*model.User, error)
	CheckOrgMembership(ctx context.Context, org, user string) (bool, error)
	RemoveOrgMember(ctx context.Context, org, user string) error

	ListOrgTeams(ctx context.Context, org string, p Paging) ([]*model.Team, error)
	CreateTeam(ctx context.Context, org string, in CreateTeamInput) (*model.Team, error)
	AddTeamMember(ctx context.Context, teamID int64, user string) error
	RemoveTeamMember(ctx context.Context, teamID int64, user string) error
	AddTeamRepository(ctx context.Context, teamID int64, org, repo string) error
	RemoveTeamRepository(ctx context.Context, teamID int64, org, repo string) error
}

// Source yields the port for one request. The use cases call it only after
// they have checked their input, so a request that is invalid is refused the
// same way whether or not the request's credential resolves.
type Source func(ctx context.Context) (Port, error)

// Paging selects a page of a list. It is the list module's paging, so a list
// use case hands it to list.Window unchanged.
type Paging = list.Paging

// CreateOrgInput is a new organization. Empty fields are left to Forgejo.
type CreateOrgInput struct {
	Name, FullName, Description, Website, Location, Visibility string
}

// EditOrgInput changes an organization. Empty fields are left alone.
type EditOrgInput struct {
	FullName, Description, Website, Location, Visibility string
}

// CreateTeamInput is a new team. An empty Permission means read.
type CreateTeamInput struct {
	Name, Description, Permission string
	CanCreateOrgRepo              bool
	IncludesAllRepositories       bool
}
