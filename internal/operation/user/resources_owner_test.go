// SPDX-License-Identifier: GPL-3.0-or-later

package user

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/resource"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
)

func readOwner(h handlers, owner string) ([]mcp.ResourceContents, error) {
	return h.ownerResourceHandler(context.Background(), mcp.ReadResourceRequest{
		Params: mcp.ReadResourceParams{URI: "forgejo://owner/" + owner},
	})
}

func ownerPayload(t *testing.T, contents []mcp.ResourceContents) ownerResourcePayload {
	t.Helper()
	if len(contents) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(contents))
	}
	block := contents[0].(mcp.TextResourceContents)
	if block.MIMEType != "application/json" {
		t.Errorf("MIME: got %q, want application/json", block.MIMEType)
	}
	var payload ownerResourcePayload
	if err := json.Unmarshal([]byte(block.Text), &payload); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	return payload
}

// failing makes the named lookups fail with the given status, whatever the
// underlying fake holds.
type failing struct {
	forge.Forgejo
	userStatus, orgStatus int
}

func (f failing) GetUserInfo(ctx context.Context, name string) (*forgejo_sdk.User, error) {
	if f.userStatus != 0 {
		return nil, &forge.StatusError{StatusCode: f.userStatus, Err: errors.New("user lookup failed")}
	}
	return f.Forgejo.GetUserInfo(ctx, name)
}

func (f failing) GetOrg(ctx context.Context, name string) (*forgejo_sdk.Organization, error) {
	if f.orgStatus != 0 {
		return nil, &forge.StatusError{StatusCode: f.orgStatus, Err: errors.New("upstream timeout")}
	}
	return f.Forgejo.GetOrg(ctx, name)
}

func TestOwnerResourceHandler_User(t *testing.T) {
	t.Parallel()
	h, f := newFixture()
	f.AddUser("alice")
	p := ownerPayload(t, mustRead(t, h, "alice"))
	if p.Kind != "user" || p.Login != "alice" {
		t.Fatalf("got %+v", p)
	}
}

func mustRead(t *testing.T, h handlers, owner string) []mcp.ResourceContents {
	t.Helper()
	c, err := readOwner(h, owner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return c
}

func TestOwnerResourceHandler_OrgFallback(t *testing.T) {
	t.Parallel()
	h, f := newFixture()
	if _, err := f.CreateOrg(context.Background(), forgejo_sdk.CreateOrgOption{
		Name: "forgejo-org", FullName: "Forgejo Community", Description: "The Forgejo org", Location: "Internet", Website: "https://forgejo.org",
	}); err != nil {
		t.Fatal(err)
	}
	p := ownerPayload(t, mustRead(t, h, "forgejo-org"))
	if p.Kind != "org" || p.Login != "forgejo-org" || p.FullName != "Forgejo Community" || p.Website != "https://forgejo.org" {
		t.Fatalf("got %+v", p)
	}
}

func TestOwnerResourceHandler_BothNotFound(t *testing.T) {
	t.Parallel()
	h, _ := newFixture()
	_, err := readOwner(h, "ghost")
	var resErr *resource.ResourceError
	if !errors.As(err, &resErr) {
		t.Fatalf("expected *resource.ResourceError, got %T: %v", err, err)
	}
	if resErr.Code != -32003 || !strings.Contains(resErr.Message, "owner not found") {
		t.Fatalf("got %+v", resErr)
	}
}

func TestOwnerResourceHandler_UserForbiddenIsNotMaskedByOrgFallback(t *testing.T) {
	t.Parallel()
	f := fake.New()
	f.AddUser("secret")
	h := handlers{forge: fake.Factory{Forgejo: failing{Forgejo: f, userStatus: http.StatusForbidden}}}
	_, err := readOwner(h, "secret")
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("expected the 403 to surface, got %v", err)
	}
}

func TestOwnerResourceHandler_OrgServerErrorSurfacesOrgError(t *testing.T) {
	t.Parallel()
	h := handlers{forge: fake.Factory{Forgejo: failing{Forgejo: fake.New(), orgStatus: http.StatusInternalServerError}}}
	_, err := readOwner(h, "flaky-org")
	if err == nil {
		t.Fatal("expected an error when the org lookup returns 500")
	}
	var resErr *resource.ResourceError
	if !errors.As(err, &resErr) {
		t.Fatalf("expected *resource.ResourceError, got %T: %v", err, err)
	}
	if strings.Contains(resErr.Message, "owner not found") || !strings.Contains(resErr.Message, "500") {
		t.Fatalf("expected the org's 500, got %q", resErr.Message)
	}
}

func TestOwnerResourceHandler_CredentialPolicyError(t *testing.T) {
	t.Parallel()
	h := handlers{forge: fake.Factory{Err: errors.New("no credential")}}
	_, err := readOwner(h, "alice")
	if err == nil || !strings.Contains(err.Error(), "no credential") {
		t.Fatalf("got %v", err)
	}
}
