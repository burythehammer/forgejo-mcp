// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"strings"
	"testing"
)

// The label tools carry the exclusive flag: a create or edit sends it, and a
// read shows what the forge stored. These go through the tools, not the
// adapter, so they pin what a caller sees.

func TestRepoLabel_ExclusiveRoundTrip(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)

	created := call(t, h.CreateRepoLabelFn, map[string]any{"owner": testOwner, "repo": testRepo, "name": "status/ready", "color": "#00ff00", "exclusive": true})
	if !strings.Contains(created, `"exclusive":true`) {
		t.Fatalf("create with exclusive true must report it stored, got %q", created)
	}
	id := labelIDIn(t, created)

	got := call(t, h.GetRepoLabelFn, map[string]any{"owner": testOwner, "repo": testRepo, "id": float64(id)})
	if !strings.Contains(got, `"exclusive":true`) {
		t.Fatalf("get must show exclusive, got %q", got)
	}

	// A rename that omits exclusive keeps the stored value.
	renamed := call(t, h.EditRepoLabelFn, map[string]any{"owner": testOwner, "repo": testRepo, "id": float64(id), "name": "status/done"})
	if !strings.Contains(renamed, `"exclusive":true`) {
		t.Fatalf("an edit that omits exclusive must keep it, got %q", renamed)
	}

	off := call(t, h.EditRepoLabelFn, map[string]any{"owner": testOwner, "repo": testRepo, "id": float64(id), "exclusive": false})
	if !strings.Contains(off, `"exclusive":false`) {
		t.Fatalf("edit to false must flip it, got %q", off)
	}

	plain := call(t, h.CreateRepoLabelFn, map[string]any{"owner": testOwner, "repo": testRepo, "name": "plain", "color": "#00ff00"})
	if !strings.Contains(plain, `"exclusive":false`) {
		t.Fatalf("create without exclusive must store false, got %q", plain)
	}

	list := call(t, h.ListRepoLabelsFn, map[string]any{"owner": testOwner, "repo": testRepo})
	if !strings.Contains(list, `"name":"plain"`) || !strings.Contains(list, `"exclusive":false`) {
		t.Fatalf("list must show exclusive per label, got %q", list)
	}
}

func TestOrgLabel_ExclusiveRoundTrip(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)

	created := call(t, h.CreateOrgLabelFn, map[string]any{"org": testOwner, "name": "prio/1", "color": "#0000ff", "exclusive": true})
	if !strings.Contains(created, `"exclusive":true`) {
		t.Fatalf("org create with exclusive true must report it stored, got %q", created)
	}
	id := labelIDIn(t, created)

	got := call(t, h.GetOrgLabelFn, map[string]any{"org": testOwner, "id": float64(id)})
	if !strings.Contains(got, `"exclusive":true`) {
		t.Fatalf("org get must show exclusive, got %q", got)
	}

	off := call(t, h.EditOrgLabelFn, map[string]any{"org": testOwner, "id": float64(id), "exclusive": false})
	if !strings.Contains(off, `"exclusive":false`) {
		t.Fatalf("org edit to false must flip it, got %q", off)
	}
}

// labelIDIn reads the numeric id out of a label write's result.
func labelIDIn(t *testing.T, out string) int64 {
	t.Helper()
	i := strings.Index(out, `"id":`)
	if i < 0 {
		t.Fatalf("no id in %q", out)
	}
	var id int64
	for _, c := range out[i+len(`"id":`):] {
		if c < '0' || c > '9' {
			break
		}
		id = id*10 + int64(c-'0')
	}
	if id == 0 {
		t.Fatalf("no id in %q", out)
	}
	return id
}
