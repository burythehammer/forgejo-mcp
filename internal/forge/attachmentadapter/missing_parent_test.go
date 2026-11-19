// SPDX-License-Identifier: GPL-3.0-or-later

package attachmentadapter

import (
	"context"
	"errors"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/attachment"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge/fake"
)

// A list under a missing issue is the shared not-found outcome, the same kind
// a missing issue gets everywhere else. The use case must see that kind, not
// an empty list.
func TestListUnderMissingIssueIsForgeNotFound(t *testing.T) {
	t.Parallel()
	f := fake.New()
	f.AddRepo("o", "r")
	_, err := New(f).List(context.Background(), "o", "r", attachment.ScopeIssue, 99)
	if !errors.Is(err, forge.ErrNotFound) {
		t.Fatalf("got %v, want forge.ErrNotFound", err)
	}
}
