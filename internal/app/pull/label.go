// SPDX-License-Identifier: GPL-3.0-or-later

package pull

import (
	"context"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/labelresolve"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
)

// catalog presents the pull port's label reads to the label resolver, in the
// resolver's own paging type. Issues and pull requests resolve label names
// through it, so the two share one rule.
type catalog struct{ port Port }

func (c catalog) ListRepoLabels(ctx context.Context, owner, repo string, p labelresolve.Paging) ([]*model.CatalogLabel, error) {
	return c.port.ListRepoLabels(ctx, owner, repo, Paging{Page: p.Page, Limit: p.Limit})
}

func (c catalog) ListOrgLabels(ctx context.Context, org string, p labelresolve.Paging) ([]*model.CatalogLabel, error) {
	return c.port.ListOrgLabels(ctx, org, Paging{Page: p.Page, Limit: p.Limit})
}

func (c catalog) MaxResponseItems(ctx context.Context) (int, bool) {
	return c.port.MaxResponseItems(ctx)
}
