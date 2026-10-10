// SPDX-License-Identifier: GPL-3.0-or-later

package fake

import (
	"context"
	"net/http"
	"sync"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

var _ forge.Orgs = (*Forgejo)(nil)

type orgState struct {
	mu         sync.Mutex
	orgs       []*orgRecord // creation order
	nextOrgID  int64
	nextTeamID int64
}

type orgRecord struct {
	org     *forgejo_sdk.Organization
	members []string
	teams   []*teamRecord
}

type teamRecord struct {
	team    *forgejo_sdk.Team
	members []string
	repos   []string // "org/repo"
}

func (s *orgState) find(name string) (*orgRecord, error) {
	for _, o := range s.orgs {
		if o.org.UserName == name {
			return o, nil
		}
	}
	return nil, notFound("organization")
}

func (s *orgState) team(id int64) (*orgRecord, *teamRecord, error) {
	for _, o := range s.orgs {
		for _, t := range o.teams {
			if t.team.ID == id {
				return o, t, nil
			}
		}
	}
	return nil, nil, notFound("team")
}

func orgContains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func orgWithout(list []string, s string) []string {
	out := make([]string, 0, len(list))
	for _, v := range list {
		if v != s {
			out = append(out, v)
		}
	}
	return out
}

// newTeam adds a team to the organization; the caller holds the lock.
func (s *orgState) newTeam(o *orgRecord, name, desc string, perm forgejo_sdk.AccessMode, canCreate, allRepos bool) *teamRecord {
	s.nextTeamID++
	t := &teamRecord{team: &forgejo_sdk.Team{
		ID: s.nextTeamID, Name: name, Description: desc, Permission: perm,
		CanCreateOrgRepo: canCreate, IncludesAllRepositories: allRepos,
		Organization: orgCopy(o.org),
	}}
	o.teams = append(o.teams, t)
	return t
}

func orgCopy(o *forgejo_sdk.Organization) *forgejo_sdk.Organization {
	c := *o
	return &c
}

// AddOrgMember makes the existing user a member of the organization. The port
// can remove members but not invite them.
func (f *Forgejo) AddOrgMember(org, login string) error {
	f.AddUser(login)
	f.orgs.mu.Lock()
	defer f.orgs.mu.Unlock()
	o, err := f.orgs.find(org)
	if err != nil {
		return err
	}
	if !orgContains(o.members, login) {
		o.members = append(o.members, login)
	}
	return nil
}

func (f *Forgejo) CreateOrg(_ context.Context, opt forgejo_sdk.CreateOrgOption) (*forgejo_sdk.Organization, error) {
	if err := opt.Validate(); err != nil {
		return nil, err
	}
	f.orgs.mu.Lock()
	defer f.orgs.mu.Unlock()
	if _, err := f.orgs.find(opt.Name); err == nil {
		return nil, statusErr(http.StatusUnprocessableEntity, "organization already exists")
	}
	vis := string(opt.Visibility)
	if vis == "" {
		vis = string(forgejo_sdk.VisibleTypePublic)
	}
	f.orgs.nextOrgID++
	o := &orgRecord{
		org: &forgejo_sdk.Organization{
			ID: f.orgs.nextOrgID, UserName: opt.Name, FullName: opt.FullName,
			Description: opt.Description, Website: opt.Website, Location: opt.Location, Visibility: vis,
		},
		members: []string{Poster}, // the creator owns the organization
	}
	f.orgs.orgs = append(f.orgs.orgs, o)
	// Forgejo gives every organization an Owners team holding its creator.
	owners := f.orgs.newTeam(o, "Owners", "", forgejo_sdk.AccessModeOwner, true, true)
	owners.members = []string{Poster}
	return orgCopy(o.org), nil
}

func (f *Forgejo) GetOrg(_ context.Context, name string) (*forgejo_sdk.Organization, error) {
	f.orgs.mu.Lock()
	defer f.orgs.mu.Unlock()
	o, err := f.orgs.find(name)
	if err != nil {
		return nil, err
	}
	return orgCopy(o.org), nil
}

func (f *Forgejo) EditOrg(_ context.Context, name string, opt forgejo_sdk.EditOrgOption) error {
	if err := opt.Validate(); err != nil {
		return err
	}
	f.orgs.mu.Lock()
	defer f.orgs.mu.Unlock()
	o, err := f.orgs.find(name)
	if err != nil {
		return err
	}
	// Fields left empty are left alone, as Forgejo does for a PATCH.
	if opt.FullName != "" {
		o.org.FullName = opt.FullName
	}
	if opt.Description != "" {
		o.org.Description = opt.Description
	}
	if opt.Website != "" {
		o.org.Website = opt.Website
	}
	if opt.Location != "" {
		o.org.Location = opt.Location
	}
	if opt.Visibility != "" {
		o.org.Visibility = string(opt.Visibility)
	}
	return nil
}

func (f *Forgejo) DeleteOrg(_ context.Context, name string) error {
	f.orgs.mu.Lock()
	defer f.orgs.mu.Unlock()
	for i, o := range f.orgs.orgs {
		if o.org.UserName == name {
			f.orgs.orgs = append(f.orgs.orgs[:i], f.orgs.orgs[i+1:]...)
			return nil
		}
	}
	return notFound("organization")
}

func (f *Forgejo) memberOrgs(login string, opt forgejo_sdk.ListOrgsOptions) []*forgejo_sdk.Organization {
	f.orgs.mu.Lock()
	defer f.orgs.mu.Unlock()
	var all []*forgejo_sdk.Organization
	for _, o := range f.orgs.orgs {
		if orgContains(o.members, login) {
			all = append(all, orgCopy(o.org))
		}
	}
	lo, hi := page(opt.ListOptions, len(all))
	return all[lo:hi]
}

func (f *Forgejo) ListMyOrgs(_ context.Context, opt forgejo_sdk.ListOrgsOptions) ([]*forgejo_sdk.Organization, error) {
	return f.memberOrgs(Poster, opt), nil
}

func (f *Forgejo) ListUserOrgs(_ context.Context, login string, opt forgejo_sdk.ListOrgsOptions) ([]*forgejo_sdk.Organization, error) {
	if !f.userExists(login) {
		return nil, notFound("user")
	}
	return f.memberOrgs(login, opt), nil
}

func (f *Forgejo) ListOrgMembers(_ context.Context, org string, opt forgejo_sdk.ListOrgMembershipOption) ([]*forgejo_sdk.User, error) {
	f.orgs.mu.Lock()
	o, err := f.orgs.find(org)
	if err != nil {
		f.orgs.mu.Unlock()
		return nil, err
	}
	names := append([]string{}, o.members...)
	f.orgs.mu.Unlock()

	lo, hi := page(opt.ListOptions, len(names))
	out := make([]*forgejo_sdk.User, 0, hi-lo)
	for _, n := range names[lo:hi] {
		out = append(out, user(n))
	}
	return out, nil
}

func (f *Forgejo) CheckOrgMembership(_ context.Context, org, login string) (bool, error) {
	f.orgs.mu.Lock()
	defer f.orgs.mu.Unlock()
	o, err := f.orgs.find(org)
	if err != nil {
		// Forgejo answers 404 for a missing org as for a non-member, and the
		// port reports both as "not a member".
		return false, nil
	}
	return orgContains(o.members, login), nil
}

func (f *Forgejo) RemoveOrgMember(_ context.Context, org, login string) error {
	if !f.userExists(login) {
		return notFound("user")
	}
	f.orgs.mu.Lock()
	defer f.orgs.mu.Unlock()
	o, err := f.orgs.find(org)
	if err != nil {
		return err
	}
	o.members = orgWithout(o.members, login)
	for _, t := range o.teams {
		t.members = orgWithout(t.members, login)
	}
	return nil
}

func (f *Forgejo) ListOrgTeams(_ context.Context, org string, opt forgejo_sdk.ListTeamsOptions) ([]*forgejo_sdk.Team, error) {
	f.orgs.mu.Lock()
	defer f.orgs.mu.Unlock()
	o, err := f.orgs.find(org)
	if err != nil {
		return nil, err
	}
	lo, hi := page(opt.ListOptions, len(o.teams))
	out := make([]*forgejo_sdk.Team, 0, hi-lo)
	for _, t := range o.teams[lo:hi] {
		c := *t.team
		out = append(out, &c)
	}
	return out, nil
}

func (f *Forgejo) CreateTeam(_ context.Context, org string, opt forgejo_sdk.CreateTeamOption) (*forgejo_sdk.Team, error) {
	if err := opt.Validate(); err != nil {
		return nil, err
	}
	f.orgs.mu.Lock()
	defer f.orgs.mu.Unlock()
	o, err := f.orgs.find(org)
	if err != nil {
		return nil, err
	}
	for _, t := range o.teams {
		if t.team.Name == opt.Name {
			return nil, statusErr(http.StatusUnprocessableEntity, "team already exists")
		}
	}
	t := f.orgs.newTeam(o, opt.Name, opt.Description, opt.Permission, opt.CanCreateOrgRepo, opt.IncludesAllRepositories)
	c := *t.team
	return &c, nil
}

func (f *Forgejo) AddTeamMember(_ context.Context, teamID int64, login string) error {
	if !f.userExists(login) {
		return notFound("user")
	}
	f.orgs.mu.Lock()
	defer f.orgs.mu.Unlock()
	_, t, err := f.orgs.team(teamID)
	if err != nil {
		return err
	}
	if !orgContains(t.members, login) {
		t.members = append(t.members, login)
	}
	return nil
}

func (f *Forgejo) RemoveTeamMember(_ context.Context, teamID int64, login string) error {
	if !f.userExists(login) {
		return notFound("user")
	}
	f.orgs.mu.Lock()
	defer f.orgs.mu.Unlock()
	_, t, err := f.orgs.team(teamID)
	if err != nil {
		return err
	}
	t.members = orgWithout(t.members, login)
	return nil
}

func (f *Forgejo) AddTeamRepository(_ context.Context, teamID int64, org, repo string) error {
	return f.teamRepo(teamID, org, repo, true)
}

func (f *Forgejo) RemoveTeamRepository(_ context.Context, teamID int64, org, repo string) error {
	return f.teamRepo(teamID, org, repo, false)
}

// teamRepo links or unlinks a repository. The team must belong to org; the
// fake keeps no repository list for organizations, so the repository itself is
// not checked.
func (f *Forgejo) teamRepo(teamID int64, org, repo string, add bool) error {
	f.orgs.mu.Lock()
	defer f.orgs.mu.Unlock()
	o, t, err := f.orgs.team(teamID)
	if err != nil {
		return err
	}
	if o.org.UserName != org {
		return notFound("team in organization")
	}
	full := org + "/" + repo
	if add {
		if !orgContains(t.repos, full) {
			t.repos = append(t.repos, full)
		}
	} else {
		t.repos = orgWithout(t.repos, full)
	}
	return nil
}
