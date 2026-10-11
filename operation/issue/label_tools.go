// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"context"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// ScopedLabel wraps forgejo_sdk.Label with a scope marker so the forge://
// label resources can tell repo- and org-scoped labels apart in a merged list.
// The label tools use app/issue's model.ScopedLabel instead.
type ScopedLabel struct {
	*forgejo_sdk.Label
	Scope string `json:"scope"`
}

// listOrgLabels pages an organization's labels and stamps each with
// scope="org". An owner that is not an organization has none (the port maps
// Forgejo's 404 to an empty list); 401/403 surface as errors.
//
// The page info comes back alongside the labels because the bounded org-labels
// resource pages: it requests exactly the caller's limit and reads it to decide
// whether more rows exist.
func listOrgLabels(ctx context.Context, p forge.Labels, org string, page, limit int) ([]ScopedLabel, forge.PageInfo, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 100
	}
	raw, info, err := p.ListOrgLabels(ctx, org, page, limit)
	if err != nil {
		return nil, info, err
	}
	out := make([]ScopedLabel, 0, len(raw))
	for _, l := range raw {
		out = append(out, ScopedLabel{Label: l, Scope: "org"})
	}
	return out, info, nil
}
