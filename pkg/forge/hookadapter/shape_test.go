// SPDX-License-Identifier: GPL-3.0-or-later

package hookadapter

import (
	"encoding/json"
	"testing"
	"time"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/model"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// TestRepoTypeMarshalsLikeSDK pins that app/model.Hook renders the same JSON as
// the SDK hook it mirrors. The SDK hides the URL from JSON, so the comparison is
// over every field the JSON carries.
func TestRepoTypeMarshalsLikeSDK(t *testing.T) {
	t.Parallel()
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	sdk := forgejo_sdk.Hook{
		ID: 9, Type: "forgejo", URL: "hidden", Config: map[string]string{"url": "https://x", "secret": "s"},
		Events: []string{"push"}, Active: true, Updated: ts, Created: ts,
	}
	want, err := json.Marshal(sdk)
	if err != nil {
		t.Fatal(err)
	}
	var repo model.Hook
	if err := json.Unmarshal(want, &repo); err != nil {
		t.Fatal(err)
	}
	got, err := json.Marshal(repo)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("repo type differs from SDK\nsdk:  %s\nrepo: %s", want, got)
	}
}
