// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"strings"
	"testing"
	"time"
)

func TestShutdownGraceDefaultsToEightSeconds(t *testing.T) {
	t.Parallel()
	cfg, _ := mustLoad(t, nil, env("FORGEJO_URL", "https://f.example.org"))
	if cfg.Listen.ShutdownGrace != 8*time.Second {
		t.Fatalf("default shutdown grace = %v, want 8s", cfg.Listen.ShutdownGrace)
	}
}

func TestShutdownGracePrecedence(t *testing.T) {
	t.Parallel()
	base := []string{"FORGEJO_URL", "https://f.example.org"}
	cases := []struct {
		name string
		args []string
		env  []string
		want time.Duration
	}{
		{"flag", []string{"-shutdown-grace", "30s"}, nil, 30 * time.Second},
		{"env", nil, []string{"FORGEJO_MCP_SHUTDOWN_GRACE", "2m"}, 2 * time.Minute},
		{"flag beats env", []string{"-shutdown-grace", "5s"}, []string{"FORGEJO_MCP_SHUTDOWN_GRACE", "2m"}, 5 * time.Second},
		{"explicit default beats env", []string{"-shutdown-grace", "8s"}, []string{"FORGEJO_MCP_SHUTDOWN_GRACE", "2m"}, 8 * time.Second},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			cfg, _ := mustLoad(t, c.args, env(append(append([]string{}, base...), c.env...)...))
			if cfg.Listen.ShutdownGrace != c.want {
				t.Fatalf("shutdown grace = %v, want %v", cfg.Listen.ShutdownGrace, c.want)
			}
		})
	}
}

func TestShutdownGraceRejectsInvalidValuesAtStartup(t *testing.T) {
	t.Parallel()
	base := []string{"FORGEJO_URL", "https://f.example.org"}
	cases := []struct {
		name string
		args []string
		env  []string
	}{
		{"zero flag", []string{"-shutdown-grace", "0s"}, nil},
		{"negative flag", []string{"-shutdown-grace", "-1s"}, nil},
		{"not a duration flag", []string{"-shutdown-grace", "soon"}, nil},
		{"bare number flag", []string{"-shutdown-grace", "30"}, nil},
		{"zero env", nil, []string{"FORGEJO_MCP_SHUTDOWN_GRACE", "0"}},
		{"not a duration env", nil, []string{"FORGEJO_MCP_SHUTDOWN_GRACE", "later"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := Load(c.args, env(append(append([]string{}, base...), c.env...)...), build)
			if err == nil {
				t.Fatal("Load accepted an invalid shutdown grace period")
			}
			if !strings.Contains(err.Error(), "shutdown grace") {
				t.Fatalf("error does not name the setting: %v", err)
			}
		})
	}
}
