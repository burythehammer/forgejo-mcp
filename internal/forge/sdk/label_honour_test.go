// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge"
)

// A write that asks for exclusive must fail when the forge reports another
// value: a server that drops the flag must not look like a success.
func TestHonoursCreate(t *testing.T) {
	t.Parallel()
	if err := honoursCreate(forge.LabelCreate{Name: "status/ready", Exclusive: true}, &forge.Label{Name: "status/ready", Exclusive: false}); err == nil {
		t.Fatal("create asked for exclusive, server stored none: want an error")
	}
	if err := honoursCreate(forge.LabelCreate{Name: "status/ready", Exclusive: true}, &forge.Label{Name: "status/ready", Exclusive: true}); err != nil {
		t.Fatalf("honoured create: %v", err)
	}
	if err := honoursCreate(forge.LabelCreate{Name: "plain"}, &forge.Label{Name: "plain"}); err != nil {
		t.Fatalf("default create: %v", err)
	}
}

func TestHonoursPatch(t *testing.T) {
	t.Parallel()
	on, off := true, false
	if err := honoursPatch(forge.LabelPatch{Exclusive: &on}, &forge.Label{ID: 3, Exclusive: false}); err == nil {
		t.Fatal("edit asked for exclusive, server stored none: want an error")
	}
	if err := honoursPatch(forge.LabelPatch{Exclusive: &off}, &forge.Label{ID: 3, Exclusive: false}); err != nil {
		t.Fatalf("honoured edit to false: %v", err)
	}
	// An edit that omits the flag has nothing to check.
	if err := honoursPatch(forge.LabelPatch{}, &forge.Label{ID: 3, Exclusive: true}); err != nil {
		t.Fatalf("edit without exclusive: %v", err)
	}
}
