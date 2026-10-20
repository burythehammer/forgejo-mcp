// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"errors"
	"net/http"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge"
)

func TestAnsweredKeepsStatusAndMessage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		status int
		body   string
		want   string // "" means no error
	}{
		{"a success status is no error", http.StatusNoContent, "", ""},
		{"a refusal with a JSON message carries the status and the message", http.StatusUnprocessableEntity, `{"message":"sha does not match"}`, "HTTP 422: sha does not match"},
		{"a non-JSON body falls back to the status alone", http.StatusBadGateway, "<html>bad gateway</html>", "HTTP 502"},
		{"a JSON body without a message falls back to the status alone", http.StatusForbidden, `{"url":"x"}`, "HTTP 403"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			err := answered(c.status, []byte(c.body), http.StatusNoContent)
			if c.want == "" {
				if err != nil {
					t.Fatalf("got %v, want no error", err)
				}
				return
			}
			if err == nil || err.Error() != c.want {
				t.Fatalf("got %v, want %q", err, c.want)
			}
			if forge.StatusCode(err) != c.status {
				t.Fatalf("status %d, want %d", forge.StatusCode(err), c.status)
			}
		})
	}
}

func TestAnsweredMatchesKindOfStatus(t *testing.T) {
	t.Parallel()
	err := answered(http.StatusNotFound, []byte(`{"message":"gone"}`), http.StatusNoContent)
	if !errors.Is(err, forge.ErrNotFound) {
		t.Fatalf("got %v, want the not-found kind", err)
	}
}

func TestContentsPathEscapesEachSegment(t *testing.T) {
	t.Parallel()
	got := contentsPath("octo", "hello", "docs/a b?.md")
	want := "/repos/octo/hello/contents/docs/a%20b%3F.md"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
