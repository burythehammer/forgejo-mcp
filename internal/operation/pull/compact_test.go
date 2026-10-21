// SPDX-License-Identifier: GPL-3.0-or-later

package pull

import (
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcptest"
)

func TestWriteResults_PullRequestDefaultIsCompact(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)

	created := call(t, h.CreatePullRequestFn, map[string]any{
		"head": "feat", "base": "main", "title": "T", "body": "CREATE-SENTINEL-BODY",
	})
	if strings.Contains(created, "CREATE-SENTINEL-BODY") {
		t.Errorf("create default echoed the submitted body: %s", created)
	}
	keys := mcptest.EnvelopeKeys(t, created)
	for _, k := range []string{"number", "url", "title", "state", "labels", "assignees", "updated_at", "head", "base"} {
		if _, ok := keys[k]; !ok {
			t.Errorf("create default missing %q: %s", k, created)
		}
	}
	for _, k := range []string{"body", "user", "html_url", "id"} {
		if _, ok := keys[k]; ok {
			t.Errorf("create default carries %q: %s", k, created)
		}
	}
}

func TestWriteResults_PullRequestVerboseIsFull(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)

	created := call(t, h.CreatePullRequestFn, map[string]any{
		"head": "feat", "base": "main", "title": "T", "body": "VERBOSE-SENTINEL-BODY", "verbose": true,
	})
	keys := mcptest.EnvelopeKeys(t, created)
	if _, ok := keys["body"]; !ok {
		t.Errorf("create verbose: no body key: %s", created)
	}
	if _, ok := keys["user"]; !ok {
		t.Errorf("create verbose: no user key: %s", created)
	}
	if !strings.Contains(created, "VERBOSE-SENTINEL-BODY") {
		t.Errorf("create verbose: body not returned: %s", created)
	}
}

func TestWriteResults_PullRequestUpdateDefaultIsCompact(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	idx := float64(openPR(t, f, "seed"))

	updated := call(t, h.UpdatePullRequestFn, map[string]any{
		"index": idx, "title": "renamed", "body": "UPDATE-SENTINEL-BODY",
	})
	if strings.Contains(updated, "UPDATE-SENTINEL-BODY") {
		t.Errorf("update default echoed the submitted body: %s", updated)
	}
	keys := mcptest.EnvelopeKeys(t, updated)
	if _, ok := keys["user"]; ok {
		t.Errorf("update default carries user: %s", updated)
	}
	if !strings.Contains(updated, "renamed") {
		t.Errorf("update default lost the new title: %s", updated)
	}

	verbose := call(t, h.UpdatePullRequestFn, map[string]any{
		"index": idx, "body": "UPDATE-SENTINEL-BODY", "verbose": true,
	})
	if _, ok := mcptest.EnvelopeKeys(t, verbose)["user"]; !ok || !strings.Contains(verbose, "UPDATE-SENTINEL-BODY") {
		t.Errorf("update verbose is not the full entity: %s", verbose)
	}
}
