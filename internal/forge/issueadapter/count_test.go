// SPDX-License-Identifier: GPL-3.0-or-later

package issueadapter_test

import (
	"context"
	"errors"
	"testing"

	appissue "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/issue"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/issueadapter"
)

func TestCountIssuesLabelled_UnseenRepoIsInaccessible(t *testing.T) {
	t.Parallel()
	port := issueadapter.New(fake.New())
	_, err := port.CountIssuesLabelled(context.Background(), "acme", "missing", "infra")
	var inaccessible *appissue.InaccessibleError
	if !errors.As(err, &inaccessible) {
		t.Fatalf("error = %v, want *InaccessibleError", err)
	}
}

func TestCountIssuesLabelled_ServerFailureIsNotInaccessible(t *testing.T) {
	t.Parallel()
	f := fake.New()
	f.AddRepo("acme", "a")
	f.FailIssues(500)
	_, err := issueadapter.New(f).CountIssuesLabelled(context.Background(), "acme", "a", "infra")
	var inaccessible *appissue.InaccessibleError
	if err == nil || errors.As(err, &inaccessible) {
		t.Fatalf("error = %v, want a plain failure", err)
	}
}
