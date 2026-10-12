// SPDX-License-Identifier: GPL-3.0-or-later

package search

import (
	"context"
	"fmt"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
)

// Service runs the search use cases over a Port. Build one per request from
// the port the request's credential selects; it holds no other state.
type Service struct{ port Port }

// New returns the use cases bound to port.
func New(port Port) *Service { return &Service{port: port} }

// Users returns the users matching q.
func (s *Service) Users(ctx context.Context, q UserQuery) ([]*model.User, error) {
	users, err := s.port.SearchUsers(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("search user err: %w", err)
	}
	return users, nil
}

// OrgTeams returns the teams of org matching q.
func (s *Service) OrgTeams(ctx context.Context, org string, q TeamQuery) ([]*model.Team, error) {
	teams, err := s.port.SearchOrgTeams(ctx, org, q)
	if err != nil {
		return nil, fmt.Errorf("search org teams err: %w", err)
	}
	return teams, nil
}

// Repos returns the repositories matching q.
func (s *Service) Repos(ctx context.Context, q RepoQuery) ([]*model.Repository, error) {
	repos, err := s.port.SearchRepos(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("search repos err: %w", err)
	}
	return repos, nil
}
