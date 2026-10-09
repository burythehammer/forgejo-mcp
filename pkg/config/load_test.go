// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"errors"
	"flag"
	"reflect"
	"strings"
	"testing"
)

func env(kv ...string) func(string) string {
	m := map[string]string{}
	for i := 0; i+1 < len(kv); i += 2 {
		m[kv[i]] = kv[i+1]
	}
	return func(k string) string { return m[k] }
}

var build = Build{Version: "9.9.9"}

func mustLoad(t *testing.T, args []string, getenv func(string) string) (Config, []Warning) {
	t.Helper()
	cfg, warns, err := Load(args, getenv, build)
	if err != nil {
		t.Fatalf("Load(%q): %v", args, err)
	}
	return cfg, warns
}

func TestLoadDefaults(t *testing.T) {
	t.Parallel()
	cfg, warns := mustLoad(t, nil, env("FORGEJO_URL", "https://f.example.org"))
	if len(warns) != 0 {
		t.Fatalf("warnings = %v", warns)
	}
	want := Config{
		Mode:      ModeServe,
		Transport: "stdio",
		Forgejo:   Forgejo{URL: "https://f.example.org", UserAgent: "forgejo-mcp/9.9.9"},
		Listen:    Listen{SSEPort: 8080, HTTPPort: 8080, Host: "localhost"},
		Auth: Auth{
			Mode:                 "passthrough",
			ScopesSupported:      []string{},
			ForgejoAudienceClaim: "forgejo_aud",
		},
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("got  %#v\nwant %#v", cfg, want)
	}
}

func TestLoadPrecedencePerSetting(t *testing.T) {
	t.Parallel()
	base := []string{"FORGEJO_URL", "https://f.example.org"}
	cases := []struct {
		name string
		args []string
		env  []string
		get  func(Config) any
		want any
	}{
		{"url flag", []string{"-url", "https://flag.example.org"}, []string{"FORGEJO_URL", "https://env.example.org"}, func(c Config) any { return c.Forgejo.URL }, "https://flag.example.org"},
		{"url env", nil, []string{"FORGEJO_URL", "https://env.example.org"}, func(c Config) any { return c.Forgejo.URL }, "https://env.example.org"},
		{"url deprecated", nil, []string{"GITEA_HOST", "https://old.example.org"}, func(c Config) any { return c.Forgejo.URL }, "https://old.example.org"},
		{"url env beats deprecated", nil, []string{"FORGEJO_URL", "https://env.example.org", "GITEA_HOST", "https://old.example.org"}, func(c Config) any { return c.Forgejo.URL }, "https://env.example.org"},
		{"token flag", []string{"-token", "f"}, append([]string{"FORGEJO_ACCESS_TOKEN", "e"}, base...), func(c Config) any { return c.Forgejo.Token.Reveal() }, "f"},
		{"token env", nil, append([]string{"FORGEJO_ACCESS_TOKEN", "e"}, base...), func(c Config) any { return c.Forgejo.Token.Reveal() }, "e"},
		{"token deprecated", nil, append([]string{"GITEA_ACCESS_TOKEN", "g"}, base...), func(c Config) any { return c.Forgejo.Token.Reveal() }, "g"},
		{"token env beats deprecated", nil, append([]string{"FORGEJO_ACCESS_TOKEN", "e", "GITEA_ACCESS_TOKEN", "g"}, base...), func(c Config) any { return c.Forgejo.Token.Reveal() }, "e"},
		{"user agent flag", []string{"-user-agent", "f"}, append([]string{"FORGEJO_USER_AGENT", "e"}, base...), func(c Config) any { return c.Forgejo.UserAgent }, "f"},
		{"user agent env", nil, append([]string{"FORGEJO_USER_AGENT", "e"}, base...), func(c Config) any { return c.Forgejo.UserAgent }, "e"},
		{"host flag beats env", []string{"-host", "127.0.0.1"}, append([]string{"FORGEJO_MCP_HOST", "0.0.0.0"}, base...), func(c Config) any { return c.Listen.Host }, "127.0.0.1"},
		{"host explicit default beats env", []string{"-host", "localhost"}, append([]string{"FORGEJO_MCP_HOST", "0.0.0.0"}, base...), func(c Config) any { return c.Listen.Host }, "localhost"},
		{"host env", nil, append([]string{"FORGEJO_MCP_HOST", "0.0.0.0"}, base...), func(c Config) any { return c.Listen.Host }, "0.0.0.0"},
		{"allowed hosts flag", []string{"-allowed-hosts", " a , b,,"}, append([]string{"FORGEJO_MCP_ALLOWED_HOSTS", "x"}, base...), func(c Config) any { return c.Listen.AllowedHosts }, []string{"a", "b"}},
		{"allowed hosts env", nil, append([]string{"FORGEJO_MCP_ALLOWED_HOSTS", "x,y"}, base...), func(c Config) any { return c.Listen.AllowedHosts }, []string{"x", "y"}},
		{"allowed origins env", nil, append([]string{"FORGEJO_MCP_ALLOWED_ORIGINS", "https://o.example.org"}, base...), func(c Config) any { return c.Listen.AllowedOrigins }, []string{"https://o.example.org"}},
		{"fallback env on", nil, append([]string{"FORGEJO_MCP_ALLOW_OPERATOR_TOKEN_FALLBACK", "YES"}, base...), func(c Config) any { return c.Listen.AllowOperatorTokenFallback }, true},
		{"fallback env garbage off", nil, append([]string{"FORGEJO_MCP_ALLOW_OPERATOR_TOKEN_FALLBACK", "maybe"}, base...), func(c Config) any { return c.Listen.AllowOperatorTokenFallback }, false},
		{"fallback explicit flag false beats env", []string{"-allow-operator-token-fallback=false"}, append([]string{"FORGEJO_MCP_ALLOW_OPERATOR_TOKEN_FALLBACK", "true"}, base...), func(c Config) any { return c.Listen.AllowOperatorTokenFallback }, false},
		{"fallback flag", []string{"-allow-operator-token-fallback"}, base, func(c Config) any { return c.Listen.AllowOperatorTokenFallback }, true},
		{"transport short flag", []string{"-t", "http"}, base, func(c Config) any { return c.Transport }, "http"},
		{"transport long flag", []string{"-transport", "sse"}, base, func(c Config) any { return c.Transport }, "sse"},
		{"ports flags", []string{"-sse-port", "1", "-http-port", "2"}, base, func(c Config) any { return [2]int{c.Listen.SSEPort, c.Listen.HTTPPort} }, [2]int{1, 2}},
		{"debug flag", []string{"-d"}, base, func(c Config) any { return c.Debug }, true},
		{"debug long flag", []string{"-debug"}, base, func(c Config) any { return c.Debug }, true},
		{"debug env", nil, append([]string{"FORGEJO_DEBUG", "true"}, base...), func(c Config) any { return c.Debug }, true},
		{"debug env other value", nil, append([]string{"FORGEJO_DEBUG", "1"}, base...), func(c Config) any { return c.Debug }, false},
		{"debug deprecated", nil, append([]string{"GITEA_DEBUG", "true"}, base...), func(c Config) any { return c.Debug }, true},
		{"auth mode env", rsTransport, append(append([]string{}, rsEnv...), base...), func(c Config) any { return c.Auth.Mode }, "resource-server"},
		{"auth mode flag beats env", []string{"-auth-mode", "passthrough"}, append([]string{"FORGEJO_MCP_AUTH_MODE", "resource-server"}, base...), func(c Config) any { return c.Auth.Mode }, "passthrough"},
		{"scopes split on whitespace", append([]string{"-scopes-supported", "a  b"}, rsTransport...), append(append([]string{}, rsEnv...), base...), func(c Config) any { return c.Auth.ScopesSupported }, []string{"a", "b"}},
		{"published keys env", rsTransport, append(append([]string{"FORGEJO_MCP_FORGEJO_JWT_PUBLISHED_KEY_FILES", "k1, k2"}, rsEnv...), base...), func(c Config) any { return c.Auth.ForgejoJWTPublishedKeyFiles }, []string{"k1", "k2"}},
		{"audience claim flag", append([]string{"-forgejo-audience-claim", "c"}, rsTransport...), append(append([]string{}, rsEnv...), base...), func(c Config) any { return c.Auth.ForgejoAudienceClaim }, "c"},
		{"mode serve", nil, base, func(c Config) any { return c.Mode }, ModeServe},
		{"mode cli", []string{"--cli", "list"}, base, func(c Config) any { return c.Mode }, ModeCLI},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			cfg, _ := mustLoad(t, c.args, env(c.env...))
			if got := c.get(cfg); !reflect.DeepEqual(got, c.want) {
				t.Fatalf("got %#v, want %#v", got, c.want)
			}
		})
	}
}

func TestLoadCLIModeReadsEnvironmentAndIgnoresTheCommandLine(t *testing.T) {
	t.Parallel()
	cfg, _ := mustLoad(t, []string{"-host", "0.0.0.0", "--cli", "get_issue", "--args", "{}"},
		env("FORGEJO_URL", "https://f.example.org", "FORGEJO_MCP_HOST", "10.0.0.1"))
	if cfg.Mode != ModeCLI || cfg.Listen.Host != "10.0.0.1" {
		t.Fatalf("mode=%v host=%q", cfg.Mode, cfg.Listen.Host)
	}
}

func TestLoadWarnsForDeprecatedVariablesOnlyWhenTheyAreUsed(t *testing.T) {
	t.Parallel()
	_, warns := mustLoad(t, nil, env("GITEA_HOST", "https://old.example.org", "GITEA_ACCESS_TOKEN", "g", "GITEA_DEBUG", "true"))
	want := []Warning{
		{Deprecated: "GITEA_HOST", Preferred: "FORGEJO_URL"},
		{Deprecated: "GITEA_ACCESS_TOKEN", Preferred: "FORGEJO_ACCESS_TOKEN"},
		{Deprecated: "GITEA_DEBUG", Preferred: "FORGEJO_DEBUG"},
	}
	if !reflect.DeepEqual(warns, want) {
		t.Fatalf("warnings = %v, want %v", warns, want)
	}
	_, warns = mustLoad(t, []string{"-url", "https://f.example.org", "-token", "x", "-d"},
		env("GITEA_HOST", "h", "GITEA_ACCESS_TOKEN", "g", "GITEA_DEBUG", "true"))
	if len(warns) != 0 {
		t.Fatalf("shadowed deprecated variables must not warn: %v", warns)
	}
	_, warns = mustLoad(t, nil, env("FORGEJO_URL", "https://f.example.org", "FORGEJO_DEBUG", "true", "GITEA_DEBUG", "true"))
	if len(warns) != 0 {
		t.Fatalf("warnings = %v", warns)
	}
}

func TestLoadErrors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		args    []string
		env     []string
		wantErr string
	}{
		{"missing url", nil, nil, "url"},
		{"bad scheme", []string{"-url", "ftp://x.example.org"}, nil, "http"},
		{"no host", []string{"-url", "https://"}, nil, "host"},
		{"unparsable", []string{"-url", "http://[::1"}, nil, "invalid URL"},
		{"unknown flag", []string{"-nope"}, []string{"FORGEJO_URL", "https://f.example.org"}, "nope"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := Load(c.args, env(c.env...), build)
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("err = %v, want containing %q", err, c.wantErr)
			}
		})
	}
}

func TestLoadHelpIsReportedWithUsage(t *testing.T) {
	t.Parallel()
	_, _, err := Load([]string{"-h"}, env(), build)
	var ue *UsageError
	if !errors.As(err, &ue) || !errors.Is(err, flag.ErrHelp) || !strings.Contains(ue.Usage, "-allowed-hosts") {
		t.Fatalf("err = %#v", err)
	}
}

func TestLoadRecordsWhichResourceServerSettingsWereSupplied(t *testing.T) {
	t.Parallel()
	cfg, _ := mustLoad(t, []string{"-t", "http", "-allowed-hosts", "mcp.example.org", "-resource", "https://mcp.example.org/mcp"},
		env("FORGEJO_URL", "https://f.example.org", "FORGEJO_MCP_AUTH_MODE", "resource-server",
			"FORGEJO_MCP_AUTHORIZATION_SERVER", "https://sso.example.org",
			"FORGEJO_MCP_FORGEJO_JWT_ISSUER", "https://mcp.example.org/issuer",
			"FORGEJO_MCP_FORGEJO_JWT_SIGNING_KEY_FILE", "/k.pem"))
	want := []string{"FORGEJO_MCP_AUTHORIZATION_SERVER", "-resource", "FORGEJO_MCP_FORGEJO_JWT_ISSUER", "FORGEJO_MCP_FORGEJO_JWT_SIGNING_KEY_FILE"}
	if got := cfg.suppliedResourceServer; !reflect.DeepEqual(got, want) {
		t.Fatalf("supplied = %v, want %v", got, want)
	}
	cfg, _ = mustLoad(t, nil, env("FORGEJO_URL", "https://f.example.org"))
	if got := cfg.suppliedResourceServer; len(got) != 0 {
		t.Fatalf("supplied = %v", got)
	}
}

func TestIsVersionRequest(t *testing.T) {
	t.Parallel()
	for args, want := range map[string]bool{"version": true, "--version": true, "-version": true, "": false, "-t": false} {
		var a []string
		if args != "" {
			a = []string{args, "extra"}
		}
		if got := IsVersionRequest(a); got != want {
			t.Errorf("IsVersionRequest(%q) = %v", a, got)
		}
	}
}

func TestLoadLeavesTheDefaultUserAgentUnsetWithoutAVersion(t *testing.T) {
	t.Parallel()
	cfg, _, err := Load(nil, env("FORGEJO_URL", "https://f.example.org"), Build{})
	if err != nil || cfg.Forgejo.UserAgent != "" {
		t.Fatalf("user agent = %q, err = %v", cfg.Forgejo.UserAgent, err)
	}
}

func TestLoadDoesNotRecordAShadowedEnvironmentVariableAsSupplied(t *testing.T) {
	t.Parallel()
	cfg, _ := mustLoad(t, []string{"-t", "http", "-host", "127.0.0.1", "-allowed-hosts", "mcp.example.org", "-forgejo-audience-claim", "forgejo_aud"}, env("FORGEJO_URL", "https://f.example.org", "FORGEJO_MCP_AUTH_MODE", "resource-server", "FORGEJO_MCP_FORGEJO_AUDIENCE_CLAIM", "other_claim", "FORGEJO_MCP_AUTHORIZATION_SERVER", "https://sso.example.org", "FORGEJO_MCP_FORGEJO_JWT_ISSUER", "https://mcp.example.org/issuer", "FORGEJO_MCP_FORGEJO_JWT_SIGNING_KEY_FILE", "/k.pem", "FORGEJO_MCP_RESOURCE", "https://mcp.example.org/mcp"))
	for _, got := range cfg.suppliedResourceServer {
		if got == "FORGEJO_MCP_FORGEJO_AUDIENCE_CLAIM" {
			t.Fatalf("shadowed variable recorded: %q", cfg.suppliedResourceServer)
		}
	}
}
