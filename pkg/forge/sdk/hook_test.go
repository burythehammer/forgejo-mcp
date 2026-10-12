// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
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

// An edit sends only the fields the caller set: an unset field must not appear
// as a key in the PATCH body, or Forgejo may read it as a value to store.
func TestEditRepoHookSendsOnlySetFields(t *testing.T) {
	t.Parallel()
	empty := ""
	off := false
	tests := []struct {
		name string
		opt  forge.HookEdit
		want map[string]any
	}{
		{"unset arguments send no keys", forge.HookEdit{}, map[string]any{}},
		{"explicit empty branch filter is sent as empty", forge.HookEdit{BranchFilter: &empty}, map[string]any{"branch_filter": ""}},
		{"explicit empty events list is sent as an empty list", forge.HookEdit{Events: []string{}}, map[string]any{"events": []any{}}},
		{
			"only the set fields are sent",
			forge.HookEdit{Active: &off, Events: []string{"push"}},
			map[string]any{"active": false, "events": []any{"push"}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			bodies := make(chan map[string]any, 1)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v1/version" {
					forgetest.WriteVersion(w)
					return
				}
				if r.Method != http.MethodPatch || r.URL.Path != "/api/v1/repos/o/r/hooks/7" {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
					http.NotFound(w, r)
					return
				}
				var got map[string]any
				if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
					t.Errorf("decoding body: %v", err)
				}
				bodies <- got
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{}`))
			}))
			t.Cleanup(srv.Close)
			c, err := forgejo_sdk.NewClient(srv.URL, forgejo_sdk.SetToken("stub-token"))
			if err != nil {
				t.Fatal(err)
			}

			if err := NewAdapter(c).WithRawBaseURL(srv.URL).EditRepoHook(context.Background(), "o", "r", 7, tc.opt); err != nil {
				t.Fatal(err)
			}
			if got := <-bodies; !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("PATCH body = %v, want %v", got, tc.want)
			}
		})
	}
}
