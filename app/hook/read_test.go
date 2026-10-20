// SPDX-License-Identifier: GPL-3.0-or-later

package hook_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	apphook "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/hook"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/model"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge/hookadapter"
)

// seedHooks creates n webhooks, each carrying the decoy secret.
func seedHooks(t *testing.T, s *apphook.Service, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		create(t, s, apphook.CreateRequest{Owner: owner, Repo: repo, URL: "https://example.test/h", Secret: secret})
	}
}

func TestReadCollection_CapsAndFlagsMore(t *testing.T) {
	t.Parallel()
	s := newService(t)
	seedHooks(t, s, 3)

	got, err := s.ReadCollection(context.Background(), owner, repo, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Hooks) != 2 || !got.Truncated {
		t.Errorf("shown=%d truncated=%v, want 2 and true", len(got.Hooks), got.Truncated)
	}
}

func TestReadCollection_ExactlyTheCapIsNotTruncated(t *testing.T) {
	t.Parallel()
	s := newService(t)
	seedHooks(t, s, 2)

	got, err := s.ReadCollection(context.Background(), owner, repo, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Hooks) != 2 || got.Truncated {
		t.Errorf("shown=%d truncated=%v, want 2 and false", len(got.Hooks), got.Truncated)
	}
}

func TestReadCollection_NeverCarriesTheSecret(t *testing.T) {
	t.Parallel()
	s := newService(t)
	seedHooks(t, s, 2)

	got, err := s.ReadCollection(context.Background(), owner, repo, 1)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), secret) {
		t.Fatalf("collection leaks the secret: %s", b)
	}
}

// failingList is a port whose webhook listing fails.
type failingList struct{ apphook.Port }

func (failingList) ListRepoHooks(context.Context, string, string, apphook.Paging) (model.HookList, error) {
	return model.HookList{}, errors.New("list boom")
}

func TestReadCollection_ListFailureIsWrapped(t *testing.T) {
	t.Parallel()
	f := fake.New()
	f.AddRepo(owner, repo)
	s := apphook.New(failingList{hookadapter.New(f)})

	_, err := s.ReadCollection(context.Background(), owner, repo, 5)
	if err == nil || !strings.Contains(err.Error(), "list boom") {
		t.Fatalf("want the listing failure, got %v", err)
	}
}
