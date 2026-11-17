// SPDX-License-Identifier: GPL-3.0-or-later

package operation

import (
	"context"
	"encoding/json"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/lean"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/toolset"
)

// trackerCoreLeanBudget caps the minified tools/list JSON-RPC response, in
// bytes, of the lean tracker-core preset. It was set from the measured size
// (10,852 bytes, about 2,713 tokens at 4 bytes a token) plus a margin of about
// 150 bytes. The design target is about 8 KB (about 2k tokens); the README
// records how far the preset is above it. Raising this budget is a review
// decision: change it only in a PR that says why the extra tool text is needed.
const trackerCoreLeanBudget = 11000

// trackerCoreLeanHeadroom is the size the preset is held to in routine work,
// about 400 bytes under the budget, so new tool text has room before the
// budget itself is at risk.
const trackerCoreLeanHeadroom = 10600

// TestTrackerCoreLeanStaysWithinBudget fails when the lean tracker-core
// selection grows past trackerCoreLeanBudget, so the standing cost cannot
// regrow unnoticed. It also fails past trackerCoreLeanHeadroom.
func TestTrackerCoreLeanStaysWithinBudget(t *testing.T) {
	sel, err := toolset.ParseWithPreset("tracker-core", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := selectedServer(t, sel)
	lean.Finish(s, true)
	req, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/list"})
	raw, err := json.Marshal(s.HandleMessage(context.Background(), req))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) > trackerCoreLeanBudget {
		t.Fatalf("lean tracker-core tools/list is %d bytes, over the %d-byte budget", len(raw), trackerCoreLeanBudget)
	}
	if len(raw) > trackerCoreLeanHeadroom {
		t.Fatalf("lean tracker-core tools/list is %d bytes, over the %d-byte headroom", len(raw), trackerCoreLeanHeadroom)
	}
}
