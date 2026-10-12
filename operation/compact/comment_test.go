// SPDX-License-Identifier: GPL-3.0-or-later

package compact

import (
	"encoding/json"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
)

func sampleComment() *model.Comment {
	return &model.Comment{
		ID:      31,
		HTMLURL: "https://forge.example/o/r/issues/7#issuecomment-31",
		Poster:  &model.User{UserName: "matt", Email: "matt@example.test", FullName: "Matt"},
		Body:    "a long comment body that must not be echoed back",
		Created: updated,
		Updated: updated,
	}
}

func TestOfComment_IDURLAndTimeOnly(t *testing.T) {
	t.Parallel()
	b, err := json.Marshal(OfComment(sampleComment()))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("compact comment has %d keys, want 3 (id, url, updated_at): %s", len(got), b)
	}
	if got["id"] != float64(31) || got["url"] != "https://forge.example/o/r/issues/7#issuecomment-31" {
		t.Fatalf("identifying fields wrong: %s", b)
	}
	if _, ok := got["updated_at"]; !ok {
		t.Fatalf("missing updated_at: %s", b)
	}
	if strings.Contains(string(b), "long comment body") || strings.Contains(string(b), "matt@example.test") {
		t.Fatalf("body or user object leaked: %s", b)
	}
}

func TestRenderComment_DefaultIsCompactVerboseIsFull(t *testing.T) {
	t.Parallel()
	res, err := RenderComment(sampleComment(), nil, false)
	if err != nil {
		t.Fatal(err)
	}
	s := sampleTextOf(t, res)
	if strings.Contains(s, "long comment body") {
		t.Fatalf("default comment result echoes the body: %s", s)
	}
	if got := keysOf(t, s); got["id"] == nil || got["url"] == nil || got["body"] != nil {
		t.Fatalf("compact comment keys wrong: %v", got)
	}

	res, err = RenderComment(sampleComment(), nil, true)
	if err != nil {
		t.Fatal(err)
	}
	got := keysOf(t, sampleTextOf(t, res))
	if got["body"] == nil || got["user"] == nil {
		t.Fatalf("verbose comment must be the full entity, keys: %v", got)
	}
}

func TestRenderComment_ErrorIsPassedThrough(t *testing.T) {
	t.Parallel()
	res, err := RenderComment(nil, errTest, false)
	if err == nil && (res == nil || !res.IsError) {
		t.Fatalf("expected an error, got res=%+v err=%v", res, err)
	}
}
