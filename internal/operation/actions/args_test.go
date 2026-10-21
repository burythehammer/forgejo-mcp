// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"math"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/mcptest"
)

// TestArgsMatchSchema keeps each tool's argument struct in agreement with its
// input schema: same names, types and required set.
func TestArgsMatchSchema(t *testing.T) {
	t.Parallel()
	mcptest.ArgsMatchSchema(t, DispatchWorkflowTool, dispatchArgs{})
	mcptest.ArgsMatchSchema(t, ListWorkflowRunsTool, listRunsArgs{})
	mcptest.ArgsMatchSchema(t, GetWorkflowRunTool, repoRunArgs{})
	mcptest.ArgsMatchSchema(t, ListActionRunJobsTool, jobsArgs{})
	mcptest.ArgsMatchSchema(t, GetActionJobLogsTool, jobLogArgs{})
	mcptest.ArgsMatchSchema(t, CancelWorkflowRunTool, repoRunArgs{})
	mcptest.ArgsMatchSchema(t, DeleteWorkflowRunTool, repoRunArgs{})
	mcptest.ArgsMatchSchema(t, ListActionRunArtifactsTool, artifactsArgs{})
	mcptest.ArgsMatchSchema(t, GetActionArtifactTool, getArtifactArgs{})
	mcptest.ArgsMatchSchema(t, ListRepoActionSecretsTool, repoSecretsListArgs{})
	mcptest.ArgsMatchSchema(t, CreateOrUpdateRepoActionSecretTool, repoSecretSetArgs{})
	mcptest.ArgsMatchSchema(t, DeleteRepoActionSecretTool, repoSecretNameArgs{})
	mcptest.ArgsMatchSchema(t, ListOrgActionSecretsTool, orgSecretsListArgs{})
	mcptest.ArgsMatchSchema(t, CreateOrUpdateOrgActionSecretTool, orgSecretSetArgs{})
	mcptest.ArgsMatchSchema(t, DeleteOrgActionSecretTool, orgSecretNameArgs{})
}

// TestNumberArgs pins how the numeric readers tell an absent value, an empty
// one, a wrongly typed one and a string holding a number apart. The tools'
// error texts depend on those cases.
func TestNumberArgs(t *testing.T) {
	t.Parallel()

	if n, ok := number("5"); !ok || n != 5 {
		t.Errorf(`number("5") = %v, %v; want 5, true`, n, ok)
	}
	if _, ok := number(true); ok {
		t.Error("number(true) reported a number")
	}
	if _, ok := number(nil); ok {
		t.Error("number(nil) reported a number")
	}

	cases := []struct {
		name     string
		v        any
		required bool
		want     int64
		wantErr  string
	}{
		{name: "absent required", v: nil, required: true, wantErr: "run_id is required"},
		{name: "empty required", v: "", required: true, wantErr: "run_id is required"},
		{name: "absent optional", v: nil, want: 0},
		{name: "wrong type required", v: true, required: true, wantErr: "run_id must be a positive integer"},
		{name: "wrong type optional", v: []any{}, wantErr: "run_id must be a positive integer"},
		{name: "string number", v: "7", required: true, want: 7},
		{name: "fraction", v: 1.5, required: true, wantErr: "run_id must be a positive integer"},
		{name: "zero", v: 0.0, required: true, wantErr: "run_id must be a positive integer"},
		{name: "huge", v: math.MaxInt64 * 2.0, required: true, wantErr: "run_id must be a positive integer"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, err := positiveIntegerArg(c.v, "run_id", c.required)
			if c.wantErr != "" {
				if err == nil || err.Error() != c.wantErr {
					t.Fatalf("error = %v; want %q", err, c.wantErr)
				}
				return
			}
			if err != nil || got != c.want {
				t.Fatalf("got %d, %v; want %d", got, err, c.want)
			}
		})
	}
}

// TestBoundedIntArg pins the default, the range and the error of a bounded
// number. A wrongly typed value is an error, not the default.
func TestBoundedIntArg(t *testing.T) {
	t.Parallel()
	if got, err := boundedIntArg(nil, "limit", 30, 1, 100); err != nil || got != 30 {
		t.Errorf("absent: got %d, %v; want 30", got, err)
	}
	if got, err := boundedIntArg("", "limit", 30, 1, 100); err != nil || got != 30 {
		t.Errorf(`empty: got %d, %v; want 30`, got, err)
	}
	if got, err := boundedIntArg("42", "limit", 30, 1, 100); err != nil || got != 42 {
		t.Errorf(`"42": got %d, %v; want 42`, got, err)
	}
	const want = "limit must be an integer between 1 and 100"
	for _, v := range []any{101.0, 0.0, 2.5, "x", false} {
		if _, err := boundedIntArg(v, "limit", 30, 1, 100); err == nil || err.Error() != want {
			t.Errorf("boundedIntArg(%v): error = %v; want %q", v, err, want)
		}
	}
}
