// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"

	apprepo "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/repo"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/repoadminadapter"
)

// handlers serve the repo tools; they hold the factory so each call uses its own request's credential.
type handlers struct{ forge forge.Factory }

// adminService is the Binder of the repository-administration tools: it looks
// up the request's credential and binds the admin use case to it. It runs only
// after the request is valid.
func (h handlers) adminService(ctx context.Context) (*apprepo.AdminService, error) {
	port, err := h.forge.For(ctx)
	if err != nil {
		return nil, err
	}
	return apprepo.NewAdmin(repoadminadapter.New(port)), nil
}
