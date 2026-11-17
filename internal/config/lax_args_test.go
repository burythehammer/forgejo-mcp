// SPDX-License-Identifier: GPL-3.0-or-later

package config

import "testing"

func TestLaxArgsDefaultsOff(t *testing.T) {
	cfg, _ := mustLoad(t, nil, env("FORGEJO_URL", toolsetURL))
	if cfg.LaxArgs {
		t.Fatal("LaxArgs = true with neither --lax-args nor FORGEJO_MCP_LAX_ARGS set")
	}
}

func TestLaxArgsFlagAndEnvironmentTurnItOn(t *testing.T) {
	cfg, _ := mustLoad(t, []string{"--lax-args"}, env("FORGEJO_URL", toolsetURL))
	if !cfg.LaxArgs {
		t.Fatal("--lax-args did not turn lax argument handling on")
	}

	cfg, _ = mustLoad(t, nil, env("FORGEJO_URL", toolsetURL, "FORGEJO_MCP_LAX_ARGS", "1"))
	if !cfg.LaxArgs {
		t.Fatal("FORGEJO_MCP_LAX_ARGS=1 did not turn lax argument handling on")
	}
}

func TestLaxArgsFlagBeatsEnvironment(t *testing.T) {
	cfg, _ := mustLoad(t, []string{"--lax-args=false"},
		env("FORGEJO_URL", toolsetURL, "FORGEJO_MCP_LAX_ARGS", "1"))
	if cfg.LaxArgs {
		t.Fatal("--lax-args=false did not beat FORGEJO_MCP_LAX_ARGS=1")
	}
}
