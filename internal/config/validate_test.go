// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"strings"
	"testing"
)

// rsEnv is a complete, valid resource-server configuration.
var rsArgs = []string{
	"-t", "http", "-auth-mode", "resource-server",
	"-authorization-server", "https://idp.example.org",
	"-resource", "https://mcp.example.org/mcp",
	"-forgejo-jwt-issuer", "https://mcp.example.org/issuer",
	"-forgejo-jwt-signing-key-file", "/keys/signing.pem",
	"-allowed-hosts", "mcp.example.org",
}

// rsEnv completes a resource-server configuration from the environment;
// rsTransport is the matching command-line part.
var (
	rsTransport = []string{"-t", "http", "-allowed-hosts", "mcp.example.org"}
	rsEnv       = []string{
		"FORGEJO_MCP_AUTH_MODE", "resource-server",
		"FORGEJO_MCP_AUTHORIZATION_SERVER", "https://idp.example.org",
		"FORGEJO_MCP_RESOURCE", "https://mcp.example.org/mcp",
		"FORGEJO_MCP_FORGEJO_JWT_ISSUER", "https://mcp.example.org/issuer",
		"FORGEJO_MCP_FORGEJO_JWT_SIGNING_KEY_FILE", "/keys/signing.pem",
	}
)

func withArgs(base []string, extra ...string) []string {
	return append(append([]string(nil), base...), extra...)
}

func TestLoadRefusesInvalidConfiguration(t *testing.T) {
	t.Parallel()
	url := []string{"FORGEJO_URL", "https://f.example.org"}
	cases := []struct {
		name string
		args []string
		env  []string
		want string
	}{
		{"unknown auth mode", []string{"-auth-mode", "bogus"}, nil, `-auth-mode "bogus" is not valid`},
		{
			"passthrough with resource flag",
			[]string{"-t", "http", "-resource", "https://x.example.org/mcp"},
			nil,
			"-resource requires -auth-mode resource-server",
		},
		{
			"passthrough with resource env",
			[]string{"-t", "http"},
			[]string{"FORGEJO_MCP_AUTHORIZATION_SERVER", "https://idp.example.org"},
			"FORGEJO_MCP_AUTHORIZATION_SERVER requires -auth-mode resource-server",
		},
		{
			"passthrough lists every supplied setting",
			[]string{"-resource", "r", "-forgejo-jwt-issuer", "i"},
			nil,
			"-resource, -forgejo-jwt-issuer requires",
		},
		{"resource-server over stdio", withArgs(rsArgs[2:]), nil, `requires the http transport, not "stdio"`},
		{"resource-server over sse", withArgs(rsArgs, "-t", "sse"), nil, `requires the http transport, not "sse"`},
		{"resource-server with --cli", []string{"--cli"}, rsEnv, "cannot be used with --cli"},
		{"resource-server with token", withArgs(rsArgs, "-token", "t"), nil, "takes no operator token"},
		{"resource-server with token env", rsArgs, []string{"FORGEJO_ACCESS_TOKEN", "t"}, "takes no operator token"},
		{
			"resource-server with fallback", withArgs(rsArgs, "-allow-operator-token-fallback"), nil,
			"cannot be combined with -auth-mode resource-server",
		},
		{
			"resource-server missing everything",
			[]string{"-t", "http", "-auth-mode", "resource-server"},
			nil,
			"requires -authorization-server, -resource, -forgejo-jwt-issuer, -forgejo-jwt-signing-key-file",
		},
		{"resource-server bad scope", withArgs(rsArgs, "-scopes-supported", `ok b"ad`), nil, "not a valid OAuth scope"},
		{
			"authorization server over http", replace(rsArgs, "https://idp.example.org", "http://idp.example.org"), nil,
			"-authorization-server",
		},
		{
			"resource wrong path", replace(rsArgs, "https://mcp.example.org/mcp", "https://mcp.example.org/other"), nil,
			`must be "/mcp"`,
		},
		{
			"issuer over http", replace(rsArgs, "https://mcp.example.org/issuer", "http://localhost/issuer"), nil,
			"must be an https URL",
		},
		{
			"issuer with trailing slash", replace(rsArgs, "https://mcp.example.org/issuer", "https://mcp.example.org/issuer/"), nil,
			"-forgejo-jwt-issuer:",
		},
		{
			"resource host not allowed", replace(rsArgs, "mcp.example.org", "other.example.org")[:len(rsArgs)-2], nil,
			"is not one this server answers to",
		},
		{
			"non-loopback bind without allowed hosts",
			[]string{"-t", "http", "-host", "0.0.0.0"},
			nil,
			"no allowed hosts are declared",
		},
		{
			"sse non-loopback bind without allowed hosts",
			[]string{"-t", "sse", "-host", "0.0.0.0"},
			nil,
			"no allowed hosts are declared",
		},
		{"empty bind address", []string{"-t", "http", "-host", " "}, nil, "empty bind address"},
		{"unusable origin", []string{"-t", "http", "-allowed-origins", "ftp://x.example.org"}, nil, "not a usable origin"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := Load(tc.args, env(append(append([]string(nil), url...), tc.env...)...), build)
			if err == nil {
				t.Fatalf("Load(%q) accepted an invalid configuration", tc.args)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want it to contain %q", err, tc.want)
			}
		})
	}
}

func replace(args []string, old, new string) []string {
	out := append([]string(nil), args...)
	for i, a := range out {
		if a == old {
			out[i] = new
		}
	}
	return out
}

func TestLoadAcceptsValidConfiguration(t *testing.T) {
	t.Parallel()
	url := []string{"FORGEJO_URL", "https://f.example.org"}
	cases := []struct {
		name string
		args []string
		env  []string
	}{
		{"defaults", nil, nil},
		{"stdio allows operator fallback and non-loopback host", []string{"-host", "0.0.0.0"}, nil},
		{"http loopback", []string{"-t", "http"}, nil},
		{"http non-loopback with allowed hosts", []string{"-t", "http", "-host", "0.0.0.0", "-allowed-hosts", "mcp.example.org"}, nil},
		{"http with origin", []string{"-t", "http", "-allowed-origins", "https://console.example.org"}, nil},
		{"explicit passthrough", []string{"-auth-mode", "passthrough"}, nil},
		{"resource-server", rsArgs, nil},
		{"resource-server from env", []string{"-t", "http", "-allowed-hosts", "mcp.example.org"}, []string{
			"FORGEJO_MCP_AUTH_MODE", "resource-server",
			"FORGEJO_MCP_AUTHORIZATION_SERVER", "https://idp.example.org",
			"FORGEJO_MCP_RESOURCE", "https://mcp.example.org/mcp",
			"FORGEJO_MCP_FORGEJO_JWT_ISSUER", "https://mcp.example.org/issuer",
			"FORGEJO_MCP_FORGEJO_JWT_SIGNING_KEY_FILE", "/keys/signing.pem",
		}},
		{"resource-server loopback http idp", []string{
			"-t", "http", "-auth-mode", "resource-server",
			"-authorization-server", "http://localhost:9000", "-resource", "http://localhost:8080/mcp",
			"-forgejo-jwt-issuer", "https://mcp.example.org/issuer", "-forgejo-jwt-signing-key-file", "/k.pem",
			"-allowed-hosts", "mcp.example.org",
		}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, _, err := Load(tc.args, env(append(append([]string(nil), url...), tc.env...)...), build); err != nil {
				t.Fatalf("Load(%q): %v", tc.args, err)
			}
		})
	}
}

func TestValidateIsArgumentFreeAndRepeatable(t *testing.T) {
	t.Parallel()
	cfg, _ := mustLoad(t, []string{"-t", "http"}, env("FORGEJO_URL", "https://f.example.org"))
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	cfg.Listen.Host = "0.0.0.0"
	if err := cfg.Validate(); err == nil {
		t.Fatal("a hand-edited Config with a public bind and no allowed hosts passed Validate")
	}
}
