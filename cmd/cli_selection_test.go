// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/toolset"
)

// TestMain lets the tests below run the real entry point in a child process:
// with FORGEJO_MCP_TEST_EXECUTE set, the test binary runs Execute with the
// arguments after "--" instead of the tests.
func TestMain(m *testing.M) {
	if os.Getenv("FORGEJO_MCP_TEST_EXECUTE") == "1" {
		Execute("test")
		return
	}
	os.Exit(m.Run())
}

// runExecute runs Execute("test") with args and dummy credentials, returning
// stdout, stderr and the exit code.
func runExecute(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = append(os.Environ(),
		"FORGEJO_MCP_TEST_EXECUTE=1",
		"FORGEJO_URL=http://127.0.0.1:1",
		"FORGEJO_TOKEN=dummy",
		"FORGEJO_MCP_TOOLSETS=", "FORGEJO_MCP_TOOLS=", "FORGEJO_MCP_PRESET=", "FORGEJO_MCP_LEAN=", "FORGEJO_MCP_LAX_ARGS=",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("running %v: %v", args, err)
	}
	return stdout.String(), stderr.String(), code
}

// listedTools returns the tool names printed by `--cli list --output=json`-less
// text output: every line indented by two spaces starts with a tool name.
func listedTools(out string) []string {
	var names []string
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "  ") {
			if f := strings.Fields(line); len(f) > 0 {
				names = append(names, f[0])
			}
		}
	}
	return names
}

func TestCLIList_HonoursSelection(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want []string
	}{
		{"toolsets", []string{"--toolsets", "issues", "--cli", "list"}, toolsetTools(t, "issues")},
		{"tools", []string{"--tools", "get_my_user_info,get_repo", "--cli", "list"}, []string{"get_my_user_info", "get_repo"}},
		{"union", []string{"--toolsets", "issues", "--tools", "get_repo", "--cli", "list"}, append(toolsetTools(t, "issues"), "get_repo")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, stderr, code := runExecute(t, tc.args...)
			if code != 0 {
				t.Fatalf("exit %d, stderr: %s", code, stderr)
			}
			got := map[string]bool{}
			for _, n := range listedTools(out) {
				got[n] = true
			}
			if len(got) != len(tc.want) {
				t.Errorf("listed %d tools, want %d", len(got), len(tc.want))
			}
			for _, n := range tc.want {
				if !got[n] {
					t.Errorf("tool %q missing from the list", n)
				}
			}
		})
	}
}

func TestCLICall_RefusesExcludedTool(t *testing.T) {
	_, stderr, code := runExecute(t, "--toolsets", "issues", "--cli", "get_my_user_info")
	if code == 0 || !strings.Contains(stderr, "unknown tool: get_my_user_info") {
		t.Errorf("exit %d, stderr %q; want a refusal of the excluded tool", code, stderr)
	}
}

func TestCLI_UnknownToolsetFailsLikeTheServer(t *testing.T) {
	_, _, code := runExecute(t, "--toolsets", "nope", "--cli", "list")
	if code == 0 {
		t.Error("an unknown toolset must fail in CLI mode")
	}
}

func TestCLI_SelectionFlagAfterCommandSaysWhereItGoes(t *testing.T) {
	_, stderr, code := runExecute(t, "--cli", "list", "--toolsets", "issues")
	if code == 0 || !strings.Contains(stderr, "before --cli") {
		t.Errorf("exit %d, stderr %q; want a message that selection flags go before --cli", code, stderr)
	}
}

func toolsetTools(t *testing.T, name string) []string {
	t.Helper()
	for _, ts := range toolset.All() {
		if ts.Name == name {
			return ts.Tools
		}
	}
	t.Fatalf("no toolset %q", name)
	return nil
}
