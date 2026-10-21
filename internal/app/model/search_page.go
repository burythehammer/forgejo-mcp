// SPDX-License-Identifier: GPL-3.0-or-later

package model

// UserSearchPage is the response of search_users: one page of matching users.
type UserSearchPage struct {
	Users []*User `json:"users"`
	PageMeta
}

// TeamSearchPage is the response of search_org_teams.
type TeamSearchPage struct {
	Teams []*Team `json:"teams"`
	PageMeta
}

// RepositoryPage is the response of search_repos.
type RepositoryPage struct {
	Repositories []*Repository `json:"repositories"`
	PageMeta
}
