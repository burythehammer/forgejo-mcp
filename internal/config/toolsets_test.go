// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"reflect"
	"strings"
	"testing"
)

const toolsetURL = "https://f.example.org"

func TestToolsetSettingsDefaultToEverything(t *testing.T) {
	cfg, _ := mustLoad(t, nil, env("FORGEJO_URL", toolsetURL))
	if len(cfg.Toolsets) != 0 || len(cfg.Tools) != 0 {
		t.Fatalf("unset selection = toolsets %v tools %v, want none", cfg.Toolsets, cfg.Tools)
	}
}

func TestToolsetSettingsResolveFromFlagsAndEnvironment(t *testing.T) {
	cfg, _ := mustLoad(t,
		[]string{"--toolsets", "issues, labels", "--tools", "get_my_user_info"},
		env("FORGEJO_URL", toolsetURL))
	if want := []string{"issues", "labels"}; !reflect.DeepEqual(cfg.Toolsets, want) {
		t.Errorf("Toolsets = %v, want %v", cfg.Toolsets, want)
	}
	if want := []string{"get_my_user_info"}; !reflect.DeepEqual(cfg.Tools, want) {
		t.Errorf("Tools = %v, want %v", cfg.Tools, want)
	}

	cfg, _ = mustLoad(t, nil, env("FORGEJO_URL", toolsetURL,
		"FORGEJO_MCP_TOOLSETS", "milestones,user", "FORGEJO_MCP_TOOLS", "list_repo_labels"))
	if want := []string{"milestones", "user"}; !reflect.DeepEqual(cfg.Toolsets, want) {
		t.Errorf("env Toolsets = %v, want %v", cfg.Toolsets, want)
	}
	if want := []string{"list_repo_labels"}; !reflect.DeepEqual(cfg.Tools, want) {
		t.Errorf("env Tools = %v, want %v", cfg.Tools, want)
	}
}

func TestToolsetFlagBeatsEnvironment(t *testing.T) {
	cfg, _ := mustLoad(t, []string{"--toolsets", "wiki"},
		env("FORGEJO_URL", toolsetURL, "FORGEJO_MCP_TOOLSETS", "issues"))
	if want := []string{"wiki"}; !reflect.DeepEqual(cfg.Toolsets, want) {
		t.Fatalf("Toolsets = %v, want the flag's %v", cfg.Toolsets, want)
	}
}

func TestUnknownToolsetFailsAtStartupNamingTheValidOnes(t *testing.T) {
	_, _, err := Load([]string{"--toolsets", "isssues"}, env("FORGEJO_URL", toolsetURL), build)
	if err == nil {
		t.Fatal("unknown toolset loaded")
	}
	if !strings.Contains(err.Error(), "isssues") || !strings.Contains(err.Error(), "labels") {
		t.Fatalf("error %q does not name the bad toolset and the valid ones", err)
	}
}

func TestUnknownToolFromEnvironmentFailsAtStartup(t *testing.T) {
	_, _, err := Load(nil, env("FORGEJO_URL", toolsetURL, "FORGEJO_MCP_TOOLS", "nope_tool"), build)
	if err == nil || !strings.Contains(err.Error(), "nope_tool") {
		t.Fatalf("unknown tool from environment: err = %v", err)
	}
}
