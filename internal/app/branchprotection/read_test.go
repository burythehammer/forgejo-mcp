// SPDX-License-Identifier: GPL-3.0-or-later

package branchprotection_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	appbp "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/branchprotection"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/branchprotectionadapter"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"
)

func TestReadCollection_CapsAndFlagsMore(t *testing.T) {
	t.Parallel()
	s := newService(t)
	for _, b := range []string{"main", "dev", "release"} {
		seed(t, s, b)
	}

	got, err := s.ReadCollection(context.Background(), owner, repo, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Rules) != 2 || !got.Truncated {
		t.Errorf("shown=%d truncated=%v, want 2 and true", len(got.Rules), got.Truncated)
	}
}

func TestReadCollection_ExactlyTheCapIsNotTruncated(t *testing.T) {
	t.Parallel()
	s := newService(t)
	seed(t, s, "main")
	seed(t, s, "dev")

	got, err := s.ReadCollection(context.Background(), owner, repo, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Rules) != 2 || got.Truncated {
		t.Errorf("shown=%d truncated=%v, want 2 and false", len(got.Rules), got.Truncated)
	}
}

// failingRules is a port whose rule listing fails.
type failingRules struct{ appbp.Port }

func (failingRules) ListBranchProtections(context.Context, string, string, appbp.Paging) ([]*model.BranchProtection, error) {
	return nil, errors.New("list boom")
}

func TestReadCollection_ListFailureIsWrapped(t *testing.T) {
	t.Parallel()
	f := fake.New()
	f.AddRepo(owner, repo)
	s := appbp.New(failingRules{branchprotectionadapter.New(f)})

	_, err := s.ReadCollection(context.Background(), owner, repo, 5)
	if err == nil || !strings.Contains(err.Error(), "list boom") {
		t.Fatalf("want the listing failure, got %v", err)
	}
}
