// SPDX-License-Identifier: GPL-3.0-or-later

package model

// UserSearchPage is the response of search_users: one page of matching users.
type UserSearchPage struct {
	Users   []*User `json:"users"`
	Page    int     `json:"page"`
	Limit   int     `json:"limit"`
	Count   int     `json:"count"`
	HasNext bool    `json:"has_next"`
}

// TeamSearchPage is the response of search_org_teams.
type TeamSearchPage struct {
	Teams   []*Team `json:"teams"`
	Page    int     `json:"page"`
	Limit   int     `json:"limit"`
	Count   int     `json:"count"`
	HasNext bool    `json:"has_next"`
}

// RepositoryPage is the response of search_repos.
type RepositoryPage struct {
	Repositories []*Repository `json:"repositories"`
	Page         int           `json:"page"`
	Limit        int           `json:"limit"`
	Count        int           `json:"count"`
	HasNext      bool          `json:"has_next"`
}
