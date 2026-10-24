// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"strings"
	"testing"
)

const presetURL = "https://f.example.org"

func TestPresetFlagResolves(t *testing.T) {
	cfg, _ := mustLoad(t, []string{"--preset", "tracker-core"}, env("FORGEJO_URL", presetURL))
	if cfg.Preset != "tracker-core" {
		t.Fatalf("Preset = %q, want tracker-core", cfg.Preset)
	}
}

func TestPresetResolvesFromEnvironment(t *testing.T) {
	cfg, _ := mustLoad(t, nil, env("FORGEJO_URL", presetURL, "FORGEJO_MCP_PRESET", "tracker-core"))
	if cfg.Preset != "tracker-core" {
		t.Fatalf("env Preset = %q, want tracker-core", cfg.Preset)
	}
}

func TestPresetFlagBeatsEnvironment(t *testing.T) {
	cfg, _ := mustLoad(t, []string{"--preset", "tracker-core"},
		env("FORGEJO_URL", presetURL, "FORGEJO_MCP_PRESET", "nothing-here"))
	if cfg.Preset != "tracker-core" {
		t.Fatalf("Preset = %q, want the flag's tracker-core", cfg.Preset)
	}
}

func TestPresetUnsetIsEmpty(t *testing.T) {
	cfg, _ := mustLoad(t, nil, env("FORGEJO_URL", presetURL))
	if cfg.Preset != "" {
		t.Fatalf("unset Preset = %q, want empty", cfg.Preset)
	}
}

func TestUnknownPresetFailsAtStartupNamingTheValidOnes(t *testing.T) {
	_, _, err := Load([]string{"--preset", "trackr-core"}, env("FORGEJO_URL", presetURL), build)
	if err == nil {
		t.Fatal("unknown preset loaded")
	}
	if !strings.Contains(err.Error(), "trackr-core") || !strings.Contains(err.Error(), "tracker-core") {
		t.Fatalf("error %q does not name the bad preset and the valid ones", err)
	}
}

func TestUnknownPresetFromEnvironmentFailsAtStartup(t *testing.T) {
	_, _, err := Load(nil, env("FORGEJO_URL", presetURL, "FORGEJO_MCP_PRESET", "nope"), build)
	if err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("unknown preset from environment: err = %v", err)
	}
}
