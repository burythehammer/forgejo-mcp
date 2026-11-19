// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge/forgetest"

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

// storedHook is what Forgejo holds for hook 7. Its branch_filter is at the top
// level, as Forgejo serves it.
const storedHook = `{"id":7,"type":"forgejo","config":{"url":"https://example.test/a","content_type":"json"},"events":["push","release"],"active":true,"branch_filter":"main"}`

// An edit sends the fields the caller set. Forgejo clears a branch filter, and
// may reset events or active, that a PATCH omits, so an unset branch_filter,
// events or active is resent with its stored value. Config and the write-only
// authorization header are sent only when set.
func TestEditRepoHookSendsOnlySetFields(t *testing.T) {
	t.Parallel()
	empty := ""
	off := false
	header := "Bearer t"
	tests := []struct {
		name string
		opt  forge.HookEdit
		want map[string]any
	}{
		{
			"unset arguments resend the stored branch filter, events and active",
			forge.HookEdit{},
			map[string]any{"branch_filter": "main", "events": []any{"push", "release"}, "active": true},
		},
		{
			"explicit empty branch filter clears it and resends the rest",
			forge.HookEdit{BranchFilter: &empty},
			map[string]any{"branch_filter": "", "events": []any{"push", "release"}, "active": true},
		},
		{
			"explicit empty events list is sent as an empty list",
			forge.HookEdit{Events: []string{}},
			map[string]any{"events": []any{}, "branch_filter": "main", "active": true},
		},
		{
			"only the set fields are sent, the rest is resent",
			forge.HookEdit{Active: &off, Events: []string{"push"}},
			map[string]any{"active": false, "events": []any{"push"}, "branch_filter": "main"},
		},
		{
			"config and authorization header are sent only when set",
			forge.HookEdit{Config: map[string]string{"url": "https://example.test/b"}, AuthorizationHeader: &header},
			map[string]any{
				"config":               map[string]any{"url": "https://example.test/b"},
				"authorization_header": "Bearer t",
				"branch_filter":        "main",
				"events":               []any{"push", "release"},
				"active":               true,
			},
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
				if r.Method == http.MethodGet && r.URL.Path == "/api/v1/repos/o/r/hooks/7" {
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(storedHook))
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
