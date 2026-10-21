// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"reflect"
	"testing"
)

func TestLoadCLIArguments(t *testing.T) {
	t.Parallel()
	e := env("FORGEJO_URL", "https://f.example.org")
	cases := []struct {
		name string
		args []string
		want CLI
	}{
		{"list defaults to text", []string{"--cli", "list"}, CLI{Tool: "list", Output: "text"}},
		{"tool defaults to json", []string{"--cli", "get_issue_by_index"}, CLI{Tool: "get_issue_by_index", Output: "json"}},
		{
			"args and output",
			[]string{"--cli", "get_issue_by_index", "--args", `{"a":1}`, "--output=text"},
			CLI{Tool: "get_issue_by_index", Args: `{"a":1}`, Output: "text"},
		},
		{"help", []string{"--cli", "get_issue_by_index", "--help"}, CLI{Tool: "get_issue_by_index", Output: "json", Help: true}},
		{"flags before --cli are ignored", []string{"-debug", "--cli", "list"}, CLI{Tool: "list", Output: "text"}},
		{"no command", []string{"--cli"}, CLI{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			cfg, _ := mustLoad(t, c.args, e)
			if cfg.Mode != ModeCLI || !reflect.DeepEqual(cfg.CLI, c.want) {
				t.Fatalf("mode=%v cli=%#v, want %#v", cfg.Mode, cfg.CLI, c.want)
			}
		})
	}
}

func TestLoadServeModeHasNoCLISettings(t *testing.T) {
	t.Parallel()
	cfg, _ := mustLoad(t, nil, env("FORGEJO_URL", "https://f.example.org"))
	if !reflect.DeepEqual(cfg.CLI, CLI{}) {
		t.Fatalf("cli = %#v", cfg.CLI)
	}
}
