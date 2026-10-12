// SPDX-License-Identifier: GPL-3.0-or-later

package search

import (
	"context"
	"fmt"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/list"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
)

// Service runs the search use cases over a Port. Build one per request from
// the port the request's credential selects; it holds no other state.
type Service struct{ port Port }

// New returns the use cases bound to port.
func New(port Port) *Service { return &Service{port: port} }

// Users returns the users matching q.
func (s *Service) Users(ctx context.Context, q UserQuery) ([]*model.User, error) {
	users, _, err := list.Window(ctx, s.port, q.Paging, defaultLimit, func(ctx context.Context, p list.Paging) ([]*model.User, error) {
		q.Paging = p
		users, err := s.port.SearchUsers(ctx, q)
		if err != nil {
			return nil, fmt.Errorf("search user err: %w", err)
		}
		return users, nil
	})
	return users, err
}

// OrgTeams returns the teams of org matching q.
func (s *Service) OrgTeams(ctx context.Context, org string, q TeamQuery) ([]*model.Team, error) {
	teams, _, err := list.Window(ctx, s.port, q.Paging, defaultLimit, func(ctx context.Context, p list.Paging) ([]*model.Team, error) {
		q.Paging = p
		teams, err := s.port.SearchOrgTeams(ctx, org, q)
		if err != nil {
			return nil, fmt.Errorf("search org teams err: %w", err)
		}
		return teams, nil
	})
	return teams, err
}

// Repos returns the repositories matching q.
func (s *Service) Repos(ctx context.Context, q RepoQuery) ([]*model.Repository, error) {
	repos, _, err := list.Window(ctx, s.port, q.Paging, defaultLimit, func(ctx context.Context, p list.Paging) ([]*model.Repository, error) {
		q.Paging = p
		repos, err := s.port.SearchRepos(ctx, q)
		if err != nil {
			return nil, fmt.Errorf("search repos err: %w", err)
		}
		return repos, nil
	})
	return repos, err
}

// defaultLimit is the page size a search uses when the caller gives none.
const defaultLimit = 100
