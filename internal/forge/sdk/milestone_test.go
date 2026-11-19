// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge/forgetest"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// A milestone edit must put only the fields the caller set on the wire. The
// SDK's EditMilestoneOption has no omitempty on its title or due date, so an
// unset title used to go out as "title":"" and an unset due date as
// "due_on":null.
func TestEditMilestoneWireBody(t *testing.T) {
	t.Parallel()
	desc := "details"
	closed := forgejo_sdk.StateClosed
	cases := []struct {
		name    string
		opt     forgejo_sdk.EditMilestoneOption
		present []string
		absent  []string
	}{
		{
			name:    "description only",
			opt:     forgejo_sdk.EditMilestoneOption{Description: &desc},
			present: []string{"description"},
			absent:  []string{"title", "state", "due_on"},
		},
		{
			name:    "state only (close)",
			opt:     forgejo_sdk.EditMilestoneOption{State: &closed},
			present: []string{"state"},
			absent:  []string{"title", "description", "due_on"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			body := patchBodyOf(t, c.opt)
			for _, k := range c.present {
				if _, ok := body[k]; !ok {
					t.Errorf("body %v: want key %q", body, k)
				}
			}
			for _, k := range c.absent {
				if v, ok := body[k]; ok {
					t.Errorf("body %v: key %q must be omitted, got %v", body, k, v)
				}
			}
		})
	}
}

// patchBodyOf sends one milestone edit through the SDK adapter to a stub that
// records the PATCH body, and returns that body decoded as a JSON object.
func patchBodyOf(t *testing.T, opt forgejo_sdk.EditMilestoneOption) map[string]any {
	t.Helper()
	var got map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/version", func(w http.ResponseWriter, _ *http.Request) { forgetest.WriteVersion(w) })
	mux.HandleFunc("PATCH /api/v1/repos/{owner}/{repo}/milestones/{id}", func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading PATCH body: %v", err)
		}
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Errorf("PATCH body %q is not a JSON object: %v", raw, err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":7,"title":"v1","state":"open"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client, err := forgejo_sdk.NewClient(srv.URL, forgejo_sdk.SetToken("stub-token"))
	if err != nil {
		t.Fatalf("building SDK client: %v", err)
	}
	a := NewAdapter(client).WithRawBaseURL(srv.URL)
	if _, err := a.EditMilestone(context.Background(), "owner", "repo", 7, opt); err != nil {
		t.Fatalf("EditMilestone: %v", err)
	}
	if got == nil {
		t.Fatal("no PATCH reached the stub")
	}
	return got
}
