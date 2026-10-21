// SPDX-License-Identifier: GPL-3.0-or-later

package operation

import (
	"context"
	"fmt"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// These tests exist because the label templates were registered as bare paths
// while their descriptions, the README table and the AGENTS.md table all
// advertised the {?page,limit} form. mcp-go matches a read against an anchored
// regexp built from the registered string, so every query-bearing read failed
// with "resource not found" before any handler ran — page and limit were
// unreachable, and so, in turn, was the paging defect underneath them.
//
// The handler tests in internal/operation/issue cannot see this: they build their own
// ReadResourceRequest and never cross the matcher. Only a real resources/read
// envelope through server.HandleMessage does.

// labelListFactory serves total labels, ids 1..total, either as the repository
// labels of o/r (scope "repo") or the labels of organization o (scope "org"). The
// fake pages, so a query-bearing URI has an observable effect rather than merely
// being accepted.
func labelListFactory(t *testing.T, scope string, total int) forge.Factory {
	t.Helper()
	f := fake.New()
	f.AddRepo("o", "r")
	ctx := context.Background()
	for i := 1; i <= total; i++ {
		opt := forgejo_sdk.CreateLabelOption{Name: fmt.Sprintf("label-%d", i), Color: "#aabbcc"}
		var err error
		if scope == "org" {
			_, err = f.CreateOrgLabel(ctx, "o", opt)
		} else {
			_, err = f.CreateRepoLabel(ctx, "o", "r", opt)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	return fake.Factory{Forgejo: f}
}

// firstLabelID reports the id of the first row, so a paged read can be shown to
// start where it should rather than merely returning the right number of rows.
func firstLabelID(t *testing.T, payload map[string]any) int {
	t.Helper()
	rows, ok := payload["labels"].([]any)
	if !ok || len(rows) == 0 {
		t.Fatalf("payload has no label rows: %#v", payload)
	}
	row, _ := rows[0].(map[string]any)
	id, _ := row["id"].(float64)
	return int(id)
}

func TestResourceDispatchRoutesRepoLabelsTemplate(t *testing.T) {
	s := server.NewMCPServer("forgejo-mcp", "test")
	RegisterCoreResources(s, labelListFactory(t, "repo", 100))

	t.Run("bare URI", func(t *testing.T) {
		payload := readResource(t, s, "forgejo://repo/o/r/labels")
		if got := rowCount(t, payload, "labels"); got != 30 {
			t.Fatalf("expected the default cap of 30 rows, got %d", got)
		}
	})

	t.Run("query-bearing URI", func(t *testing.T) {
		payload := readResource(t, s, "forgejo://repo/o/r/labels?limit=5")
		if got := rowCount(t, payload, "labels"); got != 5 {
			t.Fatalf("limit=5 did not take effect through dispatch: got %d rows", got)
		}
	})

	t.Run("query-bearing URI with page", func(t *testing.T) {
		payload := readResource(t, s, "forgejo://repo/o/r/labels?page=2&limit=5")
		if got := rowCount(t, payload, "labels"); got != 5 {
			t.Fatalf("expected 5 rows on page 2, got %d", got)
		}
		// Page 2 at limit 5 must start after page 1's last row — the whole
		// point of the paging fix.
		if got := firstLabelID(t, payload); got != 6 {
			t.Fatalf("page 2 did not start at row 6, got %d", got)
		}
	})
}

func TestResourceDispatchRoutesOrgLabelsTemplate(t *testing.T) {
	s := server.NewMCPServer("forgejo-mcp", "test")
	RegisterCoreResources(s, labelListFactory(t, "org", 100))

	t.Run("bare URI", func(t *testing.T) {
		payload := readResource(t, s, "forgejo://org/o/labels")
		if got := rowCount(t, payload, "labels"); got != 30 {
			t.Fatalf("expected the default cap of 30 rows, got %d", got)
		}
	})

	t.Run("query-bearing URI", func(t *testing.T) {
		payload := readResource(t, s, "forgejo://org/o/labels?limit=5")
		if got := rowCount(t, payload, "labels"); got != 5 {
			t.Fatalf("limit=5 did not take effect through dispatch: got %d rows", got)
		}
	})

	t.Run("query-bearing URI with page", func(t *testing.T) {
		payload := readResource(t, s, "forgejo://org/o/labels?page=2&limit=5")
		if got := rowCount(t, payload, "labels"); got != 5 {
			t.Fatalf("expected 5 rows on page 2, got %d", got)
		}
		if got := firstLabelID(t, payload); got != 6 {
			t.Fatalf("page 2 did not start at row 6, got %d", got)
		}
	})
}

// TestResourceDispatchRejectsUnknownLabelListURI keeps the query expansion from
// widening the match: adding {?…} must not turn the template into something
// that swallows neighbouring paths. Without this, "register everything as a
// prefix" would pass the tests above.
func TestResourceDispatchRejectsUnknownLabelListURI(t *testing.T) {
	s := server.NewMCPServer("forgejo-mcp", "test")
	RegisterCoreResources(s, labelListFactory(t, "repo", 10))

	for _, uri := range []string{
		"forgejo://repo/o/r/labels/extra",
		"forgejo://org/o/labels/extra",
	} {
		msg := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"resources/read","params":{"uri":%q}}`, uri)
		response := s.HandleMessage(context.Background(), []byte(msg))
		if _, isErr := response.(mcp.JSONRPCError); !isErr {
			t.Fatalf("%s should not resolve to a bounded label list resource, got %#v", uri, response)
		}
	}
}
