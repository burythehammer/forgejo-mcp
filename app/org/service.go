// SPDX-License-Identifier: GPL-3.0-or-later

package org

import (
	"context"
	"fmt"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/list"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/model"
)

// Service runs the organization, membership and team use cases. It holds only
// its Source; the port is looked up per call.
type Service struct{ src Source }

// New returns the use cases over src.
func New(src Source) *Service { return &Service{src: src} }

// permissionRead is the team permission when none is given.
const permissionRead = "read"

// CreateOrg creates an organization.
func (s *Service) CreateOrg(ctx context.Context, in CreateOrgInput) (*model.Organization, error) {
	p, err := s.port(ctx)
	if err != nil {
		return nil, err
	}
	org, err := p.CreateOrg(ctx, in)
	if err != nil {
		return nil, fmt.Errorf("create org err: %w", err)
	}
	return org, nil
}

// GetOrg returns an organization.
func (s *Service) GetOrg(ctx context.Context, org string) (*model.Organization, error) {
	p, err := s.port(ctx)
	if err != nil {
		return nil, err
	}
	o, err := p.GetOrg(ctx, org)
	if err != nil {
		return nil, fmt.Errorf("get org err: %w", err)
	}
	return o, nil
}

// EditOrg changes an organization and returns it as it now is.
func (s *Service) EditOrg(ctx context.Context, org string, in EditOrgInput) (*model.Organization, error) {
	p, err := s.port(ctx)
	if err != nil {
		return nil, err
	}
	if err := p.EditOrg(ctx, org, in); err != nil {
		return nil, fmt.Errorf("edit org err: %w", err)
	}
	o, err := p.GetOrg(ctx, org)
	if err != nil {
		return nil, fmt.Errorf("get updated org err: %w", err)
	}
	return o, nil
}

// DeleteOrg deletes an organization, with all its repositories and teams.
func (s *Service) DeleteOrg(ctx context.Context, org string) error {
	p, err := s.port(ctx)
	if err != nil {
		return err
	}
	if err := p.DeleteOrg(ctx, org); err != nil {
		return fmt.Errorf("delete org err: %w", err)
	}
	return nil
}

// defaultLimit is the page size an organization list uses when the caller gives
// none.
const defaultLimit = 100

// MyOrgs returns one page of the organizations of the credential's user.
func (s *Service) MyOrgs(ctx context.Context, pg Paging) (model.OrganizationPage, error) {
	p, err := s.port(ctx)
	if err != nil {
		return model.OrganizationPage{}, err
	}
	page, err := list.FetchPage(ctx, p, pg, defaultLimit, func(ctx context.Context, q list.Paging) ([]*model.Organization, error) {
		orgs, err := p.ListMyOrgs(ctx, q)
		if err != nil {
			return nil, fmt.Errorf("list my orgs err: %w", err)
		}
		return orgs, nil
	})
	if err != nil {
		return model.OrganizationPage{}, err
	}
	return model.OrganizationPage{Organizations: page.Items, PageMeta: model.PageMetaOf(page)}, nil
}

// UserOrgs returns one page of the organizations of a user.
func (s *Service) UserOrgs(ctx context.Context, user string, pg Paging) (model.OrganizationPage, error) {
	p, err := s.port(ctx)
	if err != nil {
		return model.OrganizationPage{}, err
	}
	page, err := list.FetchPage(ctx, p, pg, defaultLimit, func(ctx context.Context, q list.Paging) ([]*model.Organization, error) {
		orgs, err := p.ListUserOrgs(ctx, user, q)
		if err != nil {
			return nil, fmt.Errorf("list user orgs err: %w", err)
		}
		return orgs, nil
	})
	if err != nil {
		return model.OrganizationPage{}, err
	}
	return model.OrganizationPage{Organizations: page.Items, PageMeta: model.PageMetaOf(page)}, nil
}

// Members returns one page of the members of an organization.
func (s *Service) Members(ctx context.Context, org string, pg Paging) (model.MemberPage, error) {
	p, err := s.port(ctx)
	if err != nil {
		return model.MemberPage{}, err
	}
	page, err := list.FetchPage(ctx, p, pg, defaultLimit, func(ctx context.Context, q list.Paging) ([]*model.User, error) {
		members, err := p.ListOrgMembers(ctx, org, q)
		if err != nil {
			return nil, fmt.Errorf("list org members err: %w", err)
		}
		return members, nil
	})
	if err != nil {
		return model.MemberPage{}, err
	}
	return model.MemberPage{Members: page.Items, PageMeta: model.PageMetaOf(page)}, nil
}

// IsMember reports whether user is a member of org.
func (s *Service) IsMember(ctx context.Context, org, user string) (bool, error) {
	p, err := s.port(ctx)
	if err != nil {
		return false, err
	}
	ok, err := p.CheckOrgMembership(ctx, org, user)
	if err != nil {
		return false, fmt.Errorf("check org membership err: %w", err)
	}
	return ok, nil
}

// RemoveMember removes user from org.
func (s *Service) RemoveMember(ctx context.Context, org, user string) error {
	p, err := s.port(ctx)
	if err != nil {
		return err
	}
	if err := p.RemoveOrgMember(ctx, org, user); err != nil {
		return fmt.Errorf("remove org member err: %w", err)
	}
	return nil
}

// Teams returns one page of the teams of an organization.
func (s *Service) Teams(ctx context.Context, org string, pg Paging) (model.TeamPage, error) {
	p, err := s.port(ctx)
	if err != nil {
		return model.TeamPage{}, err
	}
	page, err := list.FetchPage(ctx, p, pg, defaultLimit, func(ctx context.Context, q list.Paging) ([]*model.Team, error) {
		teams, err := p.ListOrgTeams(ctx, org, q)
		if err != nil {
			return nil, fmt.Errorf("list org teams err: %w", err)
		}
		return teams, nil
	})
	if err != nil {
		return model.TeamPage{}, err
	}
	return model.TeamPage{Teams: page.Items, PageMeta: model.PageMetaOf(page)}, nil
}

// CreateTeam creates a team in an organization. An empty permission is read.
func (s *Service) CreateTeam(ctx context.Context, org string, in CreateTeamInput) (*model.Team, error) {
	if in.Permission == "" {
		in.Permission = permissionRead
	}
	p, err := s.port(ctx)
	if err != nil {
		return nil, err
	}
	team, err := p.CreateTeam(ctx, org, in)
	if err != nil {
		return nil, fmt.Errorf("create org team err: %w", err)
	}
	return team, nil
}

// AddTeamMember adds user to a team.
func (s *Service) AddTeamMember(ctx context.Context, teamID int64, user string) error {
	p, err := s.port(ctx)
	if err != nil {
		return err
	}
	if err := p.AddTeamMember(ctx, teamID, user); err != nil {
		return fmt.Errorf("add team member err: %w", err)
	}
	return nil
}

// RemoveTeamMember removes user from a team.
func (s *Service) RemoveTeamMember(ctx context.Context, teamID int64, user string) error {
	p, err := s.port(ctx)
	if err != nil {
		return err
	}
	if err := p.RemoveTeamMember(ctx, teamID, user); err != nil {
		return fmt.Errorf("remove team member err: %w", err)
	}
	return nil
}

// AddTeamRepo gives a team access to a repository.
func (s *Service) AddTeamRepo(ctx context.Context, teamID int64, org, repo string) error {
	p, err := s.port(ctx)
	if err != nil {
		return err
	}
	if err := p.AddTeamRepository(ctx, teamID, org, repo); err != nil {
		return fmt.Errorf("add team repo err: %w", err)
	}
	return nil
}

// RemoveTeamRepo withdraws a team's access to a repository.
func (s *Service) RemoveTeamRepo(ctx context.Context, teamID int64, org, repo string) error {
	p, err := s.port(ctx)
	if err != nil {
		return err
	}
	if err := p.RemoveTeamRepository(ctx, teamID, org, repo); err != nil {
		return fmt.Errorf("remove team repo err: %w", err)
	}
	return nil
}

func (s *Service) port(ctx context.Context) (Port, error) { return s.src(ctx) }
