// SPDX-License-Identifier: GPL-3.0-or-later

package issue_test

import (
	"context"
	"errors"
	"testing"

	apppissue "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/issue"
	apporg "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/org"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge/orgadapter"
)

// An org's label list follows the same rule as every list under a parent: an
// org that does not exist is not found, and an org that exists with no labels
// is an empty list. The org is created through the org use case over the same
// fake the label read uses.

func TestOrgLabels_MissingOrgIsNotFound(t *testing.T) {
	t.Parallel()
	s, _ := fixture(t)
	_, err := s.ReadOrgLabels(context.Background(), "nobody", apppissue.Paging{Page: 1, Limit: 10})
	if !errors.Is(err, forge.ErrNotFound) {
		t.Fatalf("got %v, want forge.ErrNotFound", err)
	}
}

func TestOrgLabels_EmptyOrgIsEmptyList(t *testing.T) {
	t.Parallel()
	s, f := fixture(t)
	orgs := apporg.New(func(context.Context) (apporg.Port, error) { return orgadapter.New(f), nil })
	if _, err := orgs.CreateOrg(context.Background(), apporg.CreateOrgInput{Name: "octo-org"}); err != nil {
		t.Fatalf("create org: %v", err)
	}
	out, err := s.ReadOrgLabels(context.Background(), "octo-org", apppissue.Paging{Page: 1, Limit: 10})
	if err != nil {
		t.Fatalf("got %v, want an empty list", err)
	}
	if len(out.Items) != 0 {
		t.Fatalf("got %d labels, want none", len(out.Items))
	}
}
