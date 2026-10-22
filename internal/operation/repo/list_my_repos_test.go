// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

// list_my_repos pages with optional page and limit: the schema must not mark
// them required, and a call with no arguments must take the documented defaults
// (page 1, limit 100) rather than being refused.
func TestListMyReposNoArgsUsesDefaults(t *testing.T) {
	t.Parallel()
	if req := ListMyReposTool.InputSchema.Required; len(req) != 0 {
		t.Fatalf("list_my_repos must not require arguments, schema requires %v", req)
	}

	h, _ := adminFixture(t)
	res, err := h.ListMyReposFn(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: map[string]any{}}})
	if err != nil || res == nil || res.IsError {
		t.Fatalf("no-args call failed: err=%v res=%+v", err, res)
	}

	var env struct {
		Result struct {
			Page  int `json:"page"`
			Limit int `json:"limit"`
		} `json:"Result"`
	}
	if err := json.Unmarshal([]byte(adminText(res)), &env); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if env.Result.Page != 1 || env.Result.Limit != 100 {
		t.Fatalf("defaults: page=%d limit=%d, want 1 and 100", env.Result.Page, env.Result.Limit)
	}
}
