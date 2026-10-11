// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/forgetest"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// Forgejo's X-Total-Count is advisory: a missing or unparsable one means the
// total is unknown (nil), a zero means confirmed empty.
func TestListRepoHooksTotalCountFromHeader(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		header string
		want   *int
	}{
		{"present", "4", ptrInt(4)},
		{"zero", "0", ptrInt(0)},
		{"absent", "", nil},
		{"unparsable", "garbage", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v1/version" {
					forgetest.WriteVersion(w)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				if tc.header != "" {
					w.Header().Set("X-Total-Count", tc.header)
				}
				_, _ = w.Write([]byte(`[]`))
			}))
			t.Cleanup(srv.Close)
			c, err := forgejo_sdk.NewClient(srv.URL, forgejo_sdk.SetToken("stub-token"))
			if err != nil {
				t.Fatal(err)
			}

			list, err := NewAdapter(c).ListRepoHooks(context.Background(), "o", "r", forgejo_sdk.ListHooksOptions{})
			if err != nil {
				t.Fatal(err)
			}
			switch {
			case tc.want == nil && list.TotalCount != nil:
				t.Fatalf("total = %d, want unknown", *list.TotalCount)
			case tc.want != nil && (list.TotalCount == nil || *list.TotalCount != *tc.want):
				t.Fatalf("total = %v, want %d", list.TotalCount, *tc.want)
			}
		})
	}
}

func ptrInt(n int) *int { return &n }
