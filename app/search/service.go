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

// defaultLimit is the page size a search uses when the caller gives none.
const defaultLimit = 100

// fetchPage reads one page of a search through the list module: the instance
// ceiling applies, a default limit is lowered to it, and a page below 1 is page
// 1. The continuation marker comes from the module's probe.
func fetchPage[T any](ctx context.Context, port Port, pg Paging,
	fetch func(context.Context, list.Paging) ([]T, error),
) (list.Page[T], error) {
	ceiling, known := port.MaxResponseItems(ctx)
	c := list.Ceiling{Max: ceiling, Known: known}
	return list.Run(ctx, list.Normalize(list.Paging(pg), list.DefaultLimit(defaultLimit, c)), c,
		func(ctx context.Context, q list.Paging) (list.Batch[T], error) {
			items, err := fetch(ctx, q)
			return list.Batch[T]{Items: items}, err
		})
}

// Users returns one page of the users matching q.
func (s *Service) Users(ctx context.Context, q UserQuery) (model.UserSearchPage, error) {
	page, err := fetchPage(ctx, s.port, q.Paging, func(ctx context.Context, p list.Paging) ([]*model.User, error) {
		q.Paging = p
		users, err := s.port.SearchUsers(ctx, q)
		if err != nil {
			return nil, fmt.Errorf("search user err: %w", err)
		}
		return users, nil
	})
	if err != nil {
		return model.UserSearchPage{}, err
	}
	return model.UserSearchPage{Users: page.Items, Page: page.Page, Limit: page.Limit, Count: page.Count, HasNext: page.HasNext}, nil
}

// OrgTeams returns one page of the teams of org matching q.
func (s *Service) OrgTeams(ctx context.Context, org string, q TeamQuery) (model.TeamSearchPage, error) {
	page, err := fetchPage(ctx, s.port, q.Paging, func(ctx context.Context, p list.Paging) ([]*model.Team, error) {
		q.Paging = p
		teams, err := s.port.SearchOrgTeams(ctx, org, q)
		if err != nil {
			return nil, fmt.Errorf("search org teams err: %w", err)
		}
		return teams, nil
	})
	if err != nil {
		return model.TeamSearchPage{}, err
	}
	return model.TeamSearchPage{Teams: page.Items, Page: page.Page, Limit: page.Limit, Count: page.Count, HasNext: page.HasNext}, nil
}

// Repos returns one page of the repositories matching q.
func (s *Service) Repos(ctx context.Context, q RepoQuery) (model.RepositoryPage, error) {
	page, err := fetchPage(ctx, s.port, q.Paging, func(ctx context.Context, p list.Paging) ([]*model.Repository, error) {
		q.Paging = p
		repos, err := s.port.SearchRepos(ctx, q)
		if err != nil {
			return nil, fmt.Errorf("search repos err: %w", err)
		}
		return repos, nil
	})
	if err != nil {
		return model.RepositoryPage{}, err
	}
	return model.RepositoryPage{Repositories: page.Items, Page: page.Page, Limit: page.Limit, Count: page.Count, HasNext: page.HasNext}, nil
}
