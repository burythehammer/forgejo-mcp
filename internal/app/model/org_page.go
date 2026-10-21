// SPDX-License-Identifier: GPL-3.0-or-later

package model

// OrganizationPage is the response of list_my_orgs and list_user_orgs: one page
// of organizations with the continuation signal of docs/design/output-bounding.md.
type OrganizationPage struct {
	Organizations []*Organization `json:"organizations"`
	PageMeta
}

// MemberPage is the response of list_org_members.
type MemberPage struct {
	Members []*User `json:"members"`
	PageMeta
}

// TeamPage is the response of list_org_teams.
type TeamPage struct {
	Teams []*Team `json:"teams"`
	PageMeta
}
