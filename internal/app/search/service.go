// SPDX-License-Identifier: GPL-3.0-or-later

package search

import (
	"context"
	"fmt"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/list"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
)

// Service runs the search use cases over a Port. Build one per request from
// the port the request's credential selects; it holds no other state.
type Service struct{ port Port }

// New returns the use cases bound to port.
func New(port Port) *Service { return &Service{port: port} }

// defaultLimit is the page size a search uses when the caller gives none.
const defaultLimit = 100

// Users returns one page of the users matching q.
func (s *Service) Users(ctx context.Context, q UserQuery) (list.Bounded[*model.User], error) {
	page, err := list.FetchPage(ctx, s.port, q.Paging, defaultLimit, func(ctx context.Context, p list.Paging) ([]*model.User, error) {
		q.Paging = p
		users, err := s.port.SearchUsers(ctx, q)
		if err != nil {
			return nil, fmt.Errorf("search user err: %w", err)
		}
		return users, nil
	})
	if err != nil {
		return list.Bounded[*model.User]{}, err
	}
	return page, nil
}

// OrgTeams returns one page of the teams of org matching q.
func (s *Service) OrgTeams(ctx context.Context, org string, q TeamQuery) (list.Bounded[*model.Team], error) {
	page, err := list.FetchPage(ctx, s.port, q.Paging, defaultLimit, func(ctx context.Context, p list.Paging) ([]*model.Team, error) {
		q.Paging = p
		teams, err := s.port.SearchOrgTeams(ctx, org, q)
		if err != nil {
			return nil, fmt.Errorf("search org teams err: %w", err)
		}
		return teams, nil
	})
	if err != nil {
		return list.Bounded[*model.Team]{}, err
	}
	return page, nil
}

// Repos returns one page of the repositories matching q.
func (s *Service) Repos(ctx context.Context, q RepoQuery) (list.Bounded[*model.Repository], error) {
	page, err := list.FetchPage(ctx, s.port, q.Paging, defaultLimit, func(ctx context.Context, p list.Paging) ([]*model.Repository, error) {
		q.Paging = p
		repos, err := s.port.SearchRepos(ctx, q)
		if err != nil {
			return nil, fmt.Errorf("search repos err: %w", err)
		}
		return repos, nil
	})
	if err != nil {
		return list.Bounded[*model.Repository]{}, err
	}
	return page, nil
}
