// SPDX-License-Identifier: GPL-3.0-or-later

package org

import (
	"context"
	"fmt"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
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

// MyOrgs lists the organizations of the credential's user.
func (s *Service) MyOrgs(ctx context.Context, pg Paging) ([]*model.Organization, error) {
	p, err := s.port(ctx)
	if err != nil {
		return nil, err
	}
	orgs, err := p.ListMyOrgs(ctx, pg)
	if err != nil {
		return nil, fmt.Errorf("list my orgs err: %w", err)
	}
	return orgs, nil
}

// UserOrgs lists the organizations of a user.
func (s *Service) UserOrgs(ctx context.Context, user string, pg Paging) ([]*model.Organization, error) {
	p, err := s.port(ctx)
	if err != nil {
		return nil, err
	}
	orgs, err := p.ListUserOrgs(ctx, user, pg)
	if err != nil {
		return nil, fmt.Errorf("list user orgs err: %w", err)
	}
	return orgs, nil
}

// Members lists the members of an organization.
func (s *Service) Members(ctx context.Context, org string, pg Paging) ([]*model.User, error) {
	p, err := s.port(ctx)
	if err != nil {
		return nil, err
	}
	members, err := p.ListOrgMembers(ctx, org, pg)
	if err != nil {
		return nil, fmt.Errorf("list org members err: %w", err)
	}
	return members, nil
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

// Teams lists the teams of an organization.
func (s *Service) Teams(ctx context.Context, org string, pg Paging) ([]*model.Team, error) {
	p, err := s.port(ctx)
	if err != nil {
		return nil, err
	}
	teams, err := p.ListOrgTeams(ctx, org, pg)
	if err != nil {
		return nil, fmt.Errorf("list org teams err: %w", err)
	}
	return teams, nil
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
