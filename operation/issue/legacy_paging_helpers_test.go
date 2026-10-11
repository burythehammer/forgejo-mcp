// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// Mock-server helpers for the label resource tests, which still run against
// the legacy client. The labels ticket deletes this file with them.

// paginatingHandler honours page and limit, so a test can tell whether a page
// boundary loses or repeats a row.
type paginatingHandler struct {
	total   int
	pathHas string
	row     func(i int) map[string]interface{}
}

func (h *paginatingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if !strings.Contains(r.URL.Path, h.pathHas) {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if size < 1 {
		size = 10
	}

	offset := (page - 1) * size
	rows := make([]interface{}, 0, size)
	for i := offset + 1; i <= offset+size && i <= h.total; i++ {
		rows = append(rows, h.row(i))
	}

	w.Header().Set("X-Total-Count", strconv.Itoa(h.total))
	if offset+len(rows) < h.total {
		w.Header().Set("Link", fmt.Sprintf(`<%s?page=%d&limit=%d>; rel="next"`, r.URL.Path, page+1, size))
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(rows)
}

func setupPaginatingServer(t *testing.T, h *paginatingHandler) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	client, err := forgejo_sdk.NewClient(srv.URL, forgejo_sdk.SetForgejoVersion("7.0.0"))
	if err != nil {
		t.Fatalf("creating test client: %v", err)
	}
	forgejo.SetClientForTesting(client)
	return srv
}

// assertPagesCoverCorpus walks pages 1..pages and fails if the union of what
// the client saw is not exactly the contiguous run it should be. A row that no
// page returns is the defect this guards; a row two pages both return would be
// the opposite mistake.
func assertPagesCoverCorpus(t *testing.T, seen []int, pages, limit int) {
	t.Helper()
	want := pages * limit
	if len(seen) != want {
		t.Fatalf("expected %d rows across %d pages of %d, got %d: %v", want, pages, limit, len(seen), seen)
	}
	for i, got := range seen {
		if got != i+1 {
			t.Fatalf("page boundary lost or repeated a row at position %d: want %d, got %d\nfull sequence: %v",
				i, i+1, got, seen)
		}
	}
}
