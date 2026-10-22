// SPDX-License-Identifier: GPL-3.0-or-later

package operation

import (
	"fmt"
	"testing"

	"github.com/mark3labs/mcp-go/server"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge/fake"
)

// TestResourceDispatchRoutesCommitFilePages reads the commit template through the
// server's dispatch, with and without a page query: the query has to reach the
// handler, or every read would return the first page.
func TestResourceDispatchRoutesCommitFilePages(t *testing.T) {
	const sha = "0123456789abcdef0123456789abcdef01234567"
	f := fake.New()
	f.AddRepo("o", "r")
	var paths []string
	for i := 0; i < 35; i++ {
		paths = append(paths, fmt.Sprintf("f%02d.go", i))
	}
	if err := f.AddCommit("o", "r", sha, "many files", paths...); err != nil {
		t.Fatal(err)
	}
	s := server.NewMCPServer("forgejo-mcp", "test")
	RegisterCoreResources(s, fake.Factory{Forgejo: f})

	t.Run("bare URI is the first page", func(t *testing.T) {
		payload := readResource(t, s, "forgejo://repo/o/r/commit/"+sha)
		if got := rowCount(t, payload, "files"); got != 30 {
			t.Fatalf("bare read: %d files, want the default 30", got)
		}
		if _, ok := payload["files_next"]; !ok {
			t.Fatalf("bare read has no files_next: %#v", payload)
		}
	})

	t.Run("query-bearing URI reaches the second page", func(t *testing.T) {
		payload := readResource(t, s, "forgejo://repo/o/r/commit/"+sha+"?page=2&limit=30")
		if got := rowCount(t, payload, "files"); got != 5 {
			t.Fatalf("page 2: %d files, want 5", got)
		}
		if payload["files_page"] != float64(2) {
			t.Fatalf("files_page = %v, want 2", payload["files_page"])
		}
		if _, ok := payload["files_next"]; ok {
			t.Fatalf("the last page must not name a next page: %#v", payload)
		}
	})
}
