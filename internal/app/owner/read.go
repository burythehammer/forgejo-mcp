// SPDX-License-Identifier: GPL-3.0-or-later

package owner

import (
	"context"
	"errors"
	"fmt"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
)

// Kind says whether an owner is a user or an organization.
type Kind string

const (
	KindUser Kind = "user"
	KindOrg  Kind = "org"
)

// Owner is the account a login names. Exactly one of User and Org is set, as
// Kind says.
type Owner struct {
	Kind Kind
	User *model.User
	Org  *model.Organization
}

// Service runs the owner use case over a Port. It holds no state.
type Service struct{ port Port }

// New returns the use case bound to port.
func New(port Port) *Service { return &Service{port: port} }

// Read returns the user named login. Only when no user has that login does it
// try an organization. A user lookup that fails for any reason but not-found
// is returned as it is and never masked by the organization fallback.
func (s *Service) Read(ctx context.Context, login string) (Owner, error) {
	u, userErr := s.port.GetUser(ctx, login)
	if userErr == nil && u != nil {
		return Owner{Kind: KindUser, User: u}, nil
	}
	if userErr != nil && !errors.Is(userErr, ErrNotFound) {
		return Owner{}, fmt.Errorf("get user: %w", userErr)
	}

	org, orgErr := s.port.GetOrg(ctx, login)
	if orgErr != nil {
		if errors.Is(orgErr, ErrNotFound) {
			return Owner{}, ErrOwnerNotFound
		}
		return Owner{}, fmt.Errorf("get org: %w", orgErr)
	}
	return Owner{Kind: KindOrg, Org: org}, nil
}
