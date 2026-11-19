// SPDX-License-Identifier: GPL-3.0-or-later

package branchprotection

import (
	"context"
	"fmt"
	"testing"
	"time"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/mcptest"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/resource"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// TestBranchProtectionsResourceGolden_CapAndSentinel pins the collection
// resource's 30-rule cap and its sentinel naming list_branch_protections. The
// tool lists every rule (see golden_test.go).
func TestBranchProtectionsResourceGolden_CapAndSentinel(t *testing.T) {
	resource.SetRegisteredTools([]string{ListBranchProtectionsToolName})
	t.Cleanup(func() { resource.SetRegisteredTools(nil) })

	h, f := newFixture(t)
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	f.Now = func() time.Time { return at }
	for i := 0; i < resource.EmbeddedListCap+1; i++ {
		seedRule(t, f, forgejo_sdk.CreateBranchProtectionOption{BranchName: fmt.Sprintf("b-%d", i), RuleName: fmt.Sprintf("rule-%d", i)})
	}
	contents, err := h.branchProtectionsResourceHandler(context.Background(),
		readResourceReq("forgejo://repo/goern/forgejo-mcp/branch_protections"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mcptest.Golden(t, "resource_branch_protections_cap_sentinel", []byte(resourceText(t, contents)))
}
