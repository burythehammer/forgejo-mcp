// SPDX-License-Identifier: GPL-3.0-or-later

package config

import "testing"

func TestLeanDefaultsOff(t *testing.T) {
	cfg, _ := mustLoad(t, nil, env("FORGEJO_URL", toolsetURL))
	if cfg.Lean {
		t.Fatal("Lean = true with neither --lean nor FORGEJO_MCP_LEAN set")
	}
}

func TestLeanFlagAndEnvironmentTurnItOn(t *testing.T) {
	cfg, _ := mustLoad(t, []string{"--lean"}, env("FORGEJO_URL", toolsetURL))
	if !cfg.Lean {
		t.Fatal("--lean did not turn lean mode on")
	}

	cfg, _ = mustLoad(t, nil, env("FORGEJO_URL", toolsetURL, "FORGEJO_MCP_LEAN", "1"))
	if !cfg.Lean {
		t.Fatal("FORGEJO_MCP_LEAN=1 did not turn lean mode on")
	}
}

func TestLeanFlagBeatsEnvironment(t *testing.T) {
	cfg, _ := mustLoad(t, []string{"--lean=false"},
		env("FORGEJO_URL", toolsetURL, "FORGEJO_MCP_LEAN", "1"))
	if cfg.Lean {
		t.Fatal("--lean=false did not beat FORGEJO_MCP_LEAN=1")
	}
}
