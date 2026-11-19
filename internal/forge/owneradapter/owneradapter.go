// SPDX-License-Identifier: GPL-3.0-or-later

// Package owneradapter implements the owner use case's port (internal/app/owner.Port)
// over the Forgejo port (forge.Users and forge.Orgs). It translates SDK results
// into internal/app/model values and passes a Forgejo not-found on as a 404
// *forge.StatusError, so the use case never sees an SDK type.
package owneradapter

import (
	"context"
	"fmt"
	"net/http"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/model"
	appowner "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/owner"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge/bridge"
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
		if forge.IsNotFound(err) {
			return nil, &forge.StatusError{StatusCode: http.StatusNotFound, Err: fmt.Errorf("not found: user %s", login)}
		}
		return nil, err
	}
	return bridge.Convert[*model.User](u, nil)
}

func (a adapter) GetOrg(ctx context.Context, name string) (*model.Organization, error) {
	org, err := a.o.GetOrg(ctx, name)
	if err != nil {
		if forge.IsNotFound(err) {
			return nil, &forge.StatusError{StatusCode: http.StatusNotFound, Err: fmt.Errorf("not found: org %s", name)}
		}
		return nil, err
	}
	return bridge.Convert[*model.Organization](org, nil)
}
