// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"math"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/numarg"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/mcptest"
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

// TestNumberArgWording pins how the numeric readers the tools use tell an
// absent value, an empty one, a wrongly typed one and a string holding a number
// apart. The tools' error texts depend on those cases.
func TestNumberArgWording(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		read    func(v any) (int64, error)
		v       any
		want    int64
		wantErr string
	}{
		{name: "absent required", read: required64("run_id"), v: nil, wantErr: "run_id is required"},
		{name: "empty required", read: required64("run_id"), v: "", wantErr: "run_id is required"},
		{name: "wrong type required", read: required64("run_id"), v: true, wantErr: "run_id must be a positive integer"},
		{name: "string number", read: required64("run_id"), v: "7", want: 7},
		{name: "fraction", read: required64("run_id"), v: 1.5, wantErr: "run_id must be a positive integer"},
		{name: "zero", read: required64("run_id"), v: 0.0, wantErr: "run_id must be a positive integer"},
		{name: "huge", read: required64("run_id"), v: math.MaxInt64 * 2.0, wantErr: "run_id must be a positive integer"},
		{name: "negative offset", read: nonNegative64("offset"), v: -1.0, wantErr: "offset must be a non-negative integer"},
		{name: "absent offset", read: nonNegative64("offset"), v: nil, want: 0},
		{name: "bounded absent takes default", read: bounded64("limit"), v: nil, want: 30},
		{name: "bounded empty takes default", read: bounded64("limit"), v: "", want: 30},
		{name: "bounded string number", read: bounded64("limit"), v: "42", want: 42},
		{name: "bounded above range", read: bounded64("limit"), v: 101.0, wantErr: "limit must be an integer between 1 and 100"},
		{name: "bounded fraction", read: bounded64("limit"), v: 2.5, wantErr: "limit must be an integer between 1 and 100"},
		{name: "bounded text", read: bounded64("limit"), v: "x", wantErr: "limit must be an integer between 1 and 100"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, err := c.read(c.v)
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

func required64(name string) func(any) (int64, error) {
	return func(v any) (int64, error) { return numarg.RequiredPositiveInt(name, v) }
}

func nonNegative64(name string) func(any) (int64, error) {
	return func(v any) (int64, error) {
		n, _, err := numarg.NonNegativeInt(name, v)
		return n, err
	}
}

func bounded64(name string) func(any) (int64, error) {
	return func(v any) (int64, error) {
		n, err := numarg.IntDefault(name, v, 30, numarg.Bounds{Min: 1, Max: 100})
		return int64(n), err
	}
}
