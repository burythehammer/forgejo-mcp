// SPDX-License-Identifier: GPL-3.0-or-later

// Package owneradapter implements the owner use case's port (app/owner.Port)
// over the Forgejo port (forge.Users and forge.Orgs). It translates SDK results
// into app/model values and a Forgejo not-found into owner.ErrNotFound, so the
// use case never sees an SDK type or a status code.
package owneradapter

import (
	"context"
	"fmt"
	"strings"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
	appowner "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/owner"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/bridge"
)

// New returns the owner use case's port over u and o.
func New(u forge.Users, o forge.Orgs) appowner.Port { return adapter{u: u, o: o} }

type adapter struct {
	u forge.Users
	o forge.Orgs
}

var _ appowner.Port = adapter{}

func (a adapter) GetUser(ctx context.Context, login string) (*model.User, error) {
	u, err := a.u.GetUserInfo(ctx, login)
	if err != nil {
		if isNotFound(err) {
			return nil, fmt.Errorf("%w: user %s", appowner.ErrNotFound, login)
		}
		return nil, err
	}
	return bridge.Convert[*model.User](u, nil)
}

func (a adapter) GetOrg(ctx context.Context, name string) (*model.Organization, error) {
	org, err := a.o.GetOrg(ctx, name)
	if err != nil {
		if isNotFound(err) {
			return nil, fmt.Errorf("%w: org %s", appowner.ErrNotFound, name)
		}
		return nil, err
	}
	return bridge.Convert[*model.Organization](org, nil)
}

// isNotFound is true for a Forgejo 404, or for an error whose message says so,
// as the owner resource has always judged it.
func isNotFound(err error) bool {
	if forge.IsNotFound(err) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "404") || strings.Contains(msg, "Not Found") || strings.Contains(msg, "not found")
}
