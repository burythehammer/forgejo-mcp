// SPDX-License-Identifier: GPL-3.0-or-later

package owner_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	appowner "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/owner"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
)

// stub is a Port whose lookups return fixed results and record whether the
// organization lookup ran.
type stub struct {
	user    *model.User
	userErr error
	org     *model.Organization
	orgErr  error
	orgSeen bool
}

func (s *stub) GetUser(context.Context, string) (*model.User, error) { return s.user, s.userErr }

func (s *stub) GetOrg(context.Context, string) (*model.Organization, error) {
	s.orgSeen = true
	return s.org, s.orgErr
}

func TestRead_UserWinsAndOrgIsNotAsked(t *testing.T) {
	t.Parallel()
	s := &stub{user: &model.User{UserName: "alice"}}
	got, err := appowner.New(s).Read(context.Background(), "alice")
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != appowner.KindUser || got.User.UserName != "alice" || got.Org != nil || s.orgSeen {
		t.Errorf("got %+v orgAsked=%v, want the user alone", got, s.orgSeen)
	}
}

func TestRead_NotAUserFallsBackToOrg(t *testing.T) {
	t.Parallel()
	s := &stub{userErr: forge.ErrNotFound, org: &model.Organization{UserName: "acme"}}
	got, err := appowner.New(s).Read(context.Background(), "acme")
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != appowner.KindOrg || got.Org.UserName != "acme" || got.User != nil {
		t.Errorf("got %+v, want the org", got)
	}
}

func TestRead_NeitherIsOwnerNotFound(t *testing.T) {
	t.Parallel()
	s := &stub{userErr: forge.ErrNotFound, orgErr: forge.ErrNotFound}
	_, err := appowner.New(s).Read(context.Background(), "ghost")
	if !errors.Is(err, appowner.ErrOwnerNotFound) {
		t.Fatalf("want ErrOwnerNotFound, got %v", err)
	}
}

func TestRead_UserForbiddenIsNotMaskedByTheFallback(t *testing.T) {
	t.Parallel()
	s := &stub{userErr: errors.New("403 forbidden"), org: &model.Organization{UserName: "secret"}}
	_, err := appowner.New(s).Read(context.Background(), "secret")
	if err == nil || s.orgSeen || errors.Is(err, appowner.ErrOwnerNotFound) || !strings.Contains(err.Error(), "403") {
		t.Fatalf("want the user failure and no org lookup, got err=%v orgAsked=%v", err, s.orgSeen)
	}
}

func TestRead_OrgServerErrorSurfaces(t *testing.T) {
	t.Parallel()
	s := &stub{userErr: forge.ErrNotFound, orgErr: errors.New("500 boom")}
	_, err := appowner.New(s).Read(context.Background(), "flaky")
	if err == nil || errors.Is(err, appowner.ErrOwnerNotFound) || !strings.Contains(err.Error(), "500") {
		t.Fatalf("want the org failure, got %v", err)
	}
}
