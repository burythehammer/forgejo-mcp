// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"reflect"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/config"
)

// These tests pin how arguments and environment resolve into settings, and
// what the auth checks then refuse, by driving the same entry point the binary
// does (config.Load). They characterize behaviour that must survive the move to
// a typed Config, so they assert only outcomes: the resolved setting, or the
// refusal.

const (
	decoyToken = "decoy-operator-value-not-a-credential"
	testURL    = "https://forgejo.example.org"
)

// load runs config.Load against args and a hermetic environment holding only
// FORGEJO_URL, unless env overrides it (an empty value counts as unset).
func load(args []string, env map[string]string) (config.Config, error) {
	e := map[string]string{"FORGEJO_URL": testURL}
	for k, v := range env {
		e[k] = v
	}
	cfg, _, err := config.Load(args, func(k string) string { return e[k] }, config.Build{})
	return cfg, err
}

// loadSettings loads a configuration that must be accepted.
func loadSettings(t *testing.T, args []string, env map[string]string) config.Config {
	t.Helper()
	cfg, err := load(args, env)
	if err != nil {
		t.Fatalf("load %v: %v", args, err)
	}
	return cfg
}

// resourceServerArgs is a complete, valid resource-server configuration.
func resourceServerArgs() []string {
	return []string{
		"-t", "http", "-host", "127.0.0.1",
		"-auth-mode", "resource-server",
		"-authorization-server", "https://sso.example.org",
		"-resource", "https://localhost:8443/mcp",
		"-forgejo-jwt-issuer", "https://localhost:8443/issuer",
		"-forgejo-jwt-signing-key-file", "/run/credentials/forgejo-mcp/signing.pem",
	}
}

func TestDefaultsAreLoopbackPassthroughWithNoFallback(t *testing.T) {
	cfg := loadSettings(t, nil, nil)

	if cfg.Transport != "stdio" {
		t.Errorf("transport = %q, want stdio", cfg.Transport)
	}
	if cfg.Listen.Host != "localhost" {
		t.Errorf("host = %q, want the loopback name localhost", cfg.Listen.Host)
	}
	if len(cfg.Listen.AllowedHosts) != 0 {
		t.Errorf("allowed hosts = %q, want none", cfg.Listen.AllowedHosts)
	}
	if len(cfg.Listen.AllowedOrigins) != 0 {
		t.Errorf("allowed origins = %q, want none, which accepts no browser origin", cfg.Listen.AllowedOrigins)
	}
	if cfg.Listen.AllowOperatorTokenFallback {
		t.Error("the operator-token fallback is on by default")
	}
	if cfg.Auth.Mode != "passthrough" {
		t.Errorf("auth mode = %q, want passthrough", cfg.Auth.Mode)
	}
	if cfg.Listen.SSEPort != 8080 || cfg.Listen.HTTPPort != 8080 {
		t.Errorf("ports = sse %d, http %d, want 8080 each", cfg.Listen.SSEPort, cfg.Listen.HTTPPort)
	}
	if cfg.Debug {
		t.Error("debug is on by default")
	}
}

// Each row is one setting resolved three ways. flagArgs together with envVal
// must yield flagWant (the flag beats the environment); envVal alone must
// yield envWant; neither must yield defWant. A row with skipDefault has no
// valid "neither" state.
func TestFlagBeatsEnvironmentBeatsDefaultForEachSetting(t *testing.T) {
	rows := []struct {
		name     string
		flagArgs []string
		envVar   string
		envVal   string
		// envArgs and envAlso complete the configuration when the environment
		// value alone would leave it refused.
		envArgs     []string
		envAlso     map[string]string
		get         func(config.Config) any
		flagWant    any
		envWant     any
		defWant     any
		skipDefault bool
	}{
		{
			name: "url", flagArgs: []string{"-url", "https://flag.example.org"},
			envVar: "FORGEJO_URL", envVal: "https://env.example.org",
			get:      func(cfg config.Config) any { return cfg.Forgejo.URL },
			flagWant: "https://flag.example.org", envWant: "https://env.example.org", skipDefault: true,
		},
		{
			name: "token", flagArgs: []string{"-token", "flag-" + decoyToken},
			envVar: "FORGEJO_ACCESS_TOKEN", envVal: "env-" + decoyToken,
			get:      func(cfg config.Config) any { return cfg.Forgejo.Token.Reveal() },
			flagWant: "flag-" + decoyToken, envWant: "env-" + decoyToken, defWant: "",
		},
		{
			name: "user agent", flagArgs: []string{"-user-agent", "flag-agent"},
			envVar: "FORGEJO_USER_AGENT", envVal: "env-agent",
			get:      func(cfg config.Config) any { return cfg.Forgejo.UserAgent },
			flagWant: "flag-agent", envWant: "env-agent", defWant: "",
		},
		{
			name: "host", flagArgs: []string{"-host", "127.0.0.1"},
			envVar: "FORGEJO_MCP_HOST", envVal: "0.0.0.0",
			get:      func(cfg config.Config) any { return cfg.Listen.Host },
			flagWant: "127.0.0.1", envWant: "0.0.0.0", defWant: "localhost",
		},
		{
			name: "allowed hosts", flagArgs: []string{"-allowed-hosts", "flag.example.org"},
			envVar: "FORGEJO_MCP_ALLOWED_HOSTS", envVal: "env.example.org",
			get:      func(cfg config.Config) any { return cfg.Listen.AllowedHosts },
			flagWant: []string{"flag.example.org"}, envWant: []string{"env.example.org"}, defWant: []string(nil),
		},
		{
			name: "allowed origins", flagArgs: []string{"-allowed-origins", "https://flag.example.org"},
			envVar: "FORGEJO_MCP_ALLOWED_ORIGINS", envVal: "https://env.example.org",
			get:      func(cfg config.Config) any { return cfg.Listen.AllowedOrigins },
			flagWant: []string{"https://flag.example.org"}, envWant: []string{"https://env.example.org"}, defWant: []string(nil),
		},
		{
			// An explicit false must beat an environment variable asking for true.
			name: "operator token fallback", flagArgs: []string{"-allow-operator-token-fallback=false"},
			envVar: "FORGEJO_MCP_ALLOW_OPERATOR_TOKEN_FALLBACK", envVal: "true",
			get:      func(cfg config.Config) any { return cfg.Listen.AllowOperatorTokenFallback },
			flagWant: false, envWant: true, defWant: false,
		},
		{
			// An explicit passthrough must beat an environment variable asking for
			// resource-server.
			name: "auth mode", flagArgs: []string{"-auth-mode", "passthrough"},
			envVar: "FORGEJO_MCP_AUTH_MODE", envVal: "resource-server",
			envArgs: []string{"-t", "http", "-host", "127.0.0.1"},
			envAlso: map[string]string{
				"FORGEJO_MCP_AUTHORIZATION_SERVER":         "https://sso.example.org",
				"FORGEJO_MCP_RESOURCE":                     "https://localhost:8443/mcp",
				"FORGEJO_MCP_FORGEJO_JWT_ISSUER":           "https://localhost:8443/issuer",
				"FORGEJO_MCP_FORGEJO_JWT_SIGNING_KEY_FILE": "signing.pem",
			},
			get:      func(cfg config.Config) any { return cfg.Auth.Mode },
			flagWant: "passthrough", envWant: "resource-server", defWant: "passthrough",
		},
	}
	for _, r := range rows {
		t.Run(r.name+"/flag beats environment", func(t *testing.T) {
			cfg := loadSettings(t, r.flagArgs, map[string]string{r.envVar: r.envVal})
			if got := r.get(cfg); !reflect.DeepEqual(got, r.flagWant) {
				t.Fatalf("got %#v, want the flag value %#v", got, r.flagWant)
			}
		})
		t.Run(r.name+"/environment beats default", func(t *testing.T) {
			env := map[string]string{r.envVar: r.envVal}
			for k, v := range r.envAlso {
				env[k] = v
			}
			cfg := loadSettings(t, r.envArgs, env)
			if got := r.get(cfg); !reflect.DeepEqual(got, r.envWant) {
				t.Fatalf("got %#v, want the environment value %#v", got, r.envWant)
			}
		})
		if r.skipDefault {
			continue
		}
		t.Run(r.name+"/default", func(t *testing.T) {
			cfg := loadSettings(t, nil, nil)
			if got := r.get(cfg); !reflect.DeepEqual(got, r.defWant) {
				t.Fatalf("got %#v, want the default %#v", got, r.defWant)
			}
		})
	}
}

func TestPortsAndTransportComeFromFlagsAlone(t *testing.T) {
	// There is no environment variable for these; an inherited one of a
	// plausible name must not move them.
	t.Setenv("FORGEJO_MCP_HTTP_PORT", "9999")
	t.Setenv("FORGEJO_MCP_TRANSPORT", "http")
	cfg := loadSettings(t, []string{"-t", "sse", "-sse-port", "9090", "-http-port", "9091"}, nil)

	if cfg.Transport != "sse" || cfg.Listen.SSEPort != 9090 || cfg.Listen.HTTPPort != 9091 {
		t.Fatalf("transport %q, sse port %d, http port %d", cfg.Transport, cfg.Listen.SSEPort, cfg.Listen.HTTPPort)
	}
	cfg = loadSettings(t, nil, nil)
	if cfg.Transport != "stdio" || cfg.Listen.SSEPort != 8080 || cfg.Listen.HTTPPort != 8080 {
		t.Fatalf("defaults moved: transport %q, sse port %d, http port %d", cfg.Transport, cfg.Listen.SSEPort, cfg.Listen.HTTPPort)
	}
}

func TestDebugComesFromTheFlagOrTheEnvironment(t *testing.T) {
	if cfg := loadSettings(t, []string{"-debug"}, nil); !cfg.Debug {
		t.Error("-debug did not enable debug")
	}
	if cfg := loadSettings(t, nil, map[string]string{"FORGEJO_DEBUG": "true"}); !cfg.Debug {
		t.Error("FORGEJO_DEBUG=true did not enable debug")
	}
	if cfg := loadSettings(t, nil, map[string]string{"FORGEJO_DEBUG": "false"}); cfg.Debug {
		t.Error("FORGEJO_DEBUG=false enabled debug")
	}
}

func TestDeprecatedGiteaVariablesStillWorkAndLoseToForgejoOnes(t *testing.T) {
	cfg := loadSettings(t, nil, map[string]string{
		"FORGEJO_URL": "", "GITEA_HOST": "https://gitea.example.org",
		"GITEA_ACCESS_TOKEN": "gitea-" + decoyToken, "GITEA_DEBUG": "true",
	})
	if cfg.Forgejo.URL != "https://gitea.example.org" || cfg.Forgejo.Token.Reveal() != "gitea-"+decoyToken || !cfg.Debug {
		t.Fatalf("deprecated fallbacks lost: url %q, token set %t, debug %t",
			cfg.Forgejo.URL, cfg.Forgejo.Token.Reveal() != "", cfg.Debug)
	}

	cfg = loadSettings(t, nil, map[string]string{
		"FORGEJO_URL": "https://forgejo.example.org", "GITEA_HOST": "https://gitea.example.org",
		"FORGEJO_ACCESS_TOKEN": "forgejo-" + decoyToken, "GITEA_ACCESS_TOKEN": "gitea-" + decoyToken,
	})
	if cfg.Forgejo.URL != "https://forgejo.example.org" || cfg.Forgejo.Token.Reveal() != "forgejo-"+decoyToken {
		t.Fatalf("a deprecated variable beat its replacement: url %q", cfg.Forgejo.URL)
	}
}

// Passthrough refusing resource-server-only settings is covered per setting in
// auth_config_test.go; this proves the same refusal through the full load, for
// a setting given by flag and by environment variable, on every transport.
func TestPassthroughRefusesAResourceServerSettingOnEveryTransport(t *testing.T) {
	for _, tr := range []string{"stdio", "sse", "http"} {
		t.Run(tr+"/flag", func(t *testing.T) {
			_, err := load([]string{"-t", tr, "-forgejo-jwt-issuer", "https://mcp.example.org/issuer"}, nil)
			if err == nil || !strings.Contains(err.Error(), "-forgejo-jwt-issuer") {
				t.Fatalf("error = %v, want a refusal naming the flag", err)
			}
		})
		t.Run(tr+"/environment", func(t *testing.T) {
			_, err := load([]string{"-t", tr}, map[string]string{"FORGEJO_MCP_RESOURCE": "https://mcp.example.org/mcp"})
			if err == nil || !strings.Contains(err.Error(), "FORGEJO_MCP_RESOURCE") {
				t.Fatalf("error = %v, want a refusal naming the variable", err)
			}
		})
	}
}

func TestResourceServerAcceptsACompleteConfiguration(t *testing.T) {
	// The control for the refusals below: each of them changes one thing in
	// this configuration.
	if _, err := load(resourceServerArgs(), nil); err != nil {
		t.Fatalf("a complete resource-server configuration was refused: %v", err)
	}
}

func TestResourceServerRefusals(t *testing.T) {
	cases := []struct {
		name  string
		args  []string
		env   map[string]string
		wants []string
	}{
		{
			name: "a configured token by flag", args: append(resourceServerArgs(), "-token", decoyToken),
			wants: []string{"takes no operator token"},
		},
		{
			name: "a configured token by environment", args: resourceServerArgs(),
			env:   map[string]string{"FORGEJO_ACCESS_TOKEN": decoyToken},
			wants: []string{"takes no operator token"},
		},
		{
			name: "the operator fallback by flag", args: append(resourceServerArgs(), "-allow-operator-token-fallback"),
			wants: []string{"-allow-operator-token-fallback"},
		},
		{
			name: "the operator fallback by environment", args: resourceServerArgs(),
			env:   map[string]string{"FORGEJO_MCP_ALLOW_OPERATOR_TOKEN_FALLBACK": "true"},
			wants: []string{"-allow-operator-token-fallback"},
		},
		{
			name: "the sse transport", args: append(resourceServerArgs(), "-t", "sse"),
			wants: []string{"requires the http transport"},
		},
		{
			name: "the stdio transport", args: append(resourceServerArgs(), "-t", "stdio"),
			wants: []string{"requires the http transport"},
		},
		{
			// --cli never reads flags, so the environment alone configures it.
			name: "cli mode", args: []string{"--cli", "list"},
			env: map[string]string{
				"FORGEJO_MCP_AUTH_MODE":                    "resource-server",
				"FORGEJO_MCP_AUTHORIZATION_SERVER":         "https://sso.example.org",
				"FORGEJO_MCP_RESOURCE":                     "https://localhost:8443/mcp",
				"FORGEJO_MCP_FORGEJO_JWT_ISSUER":           "https://localhost:8443/issuer",
				"FORGEJO_MCP_FORGEJO_JWT_SIGNING_KEY_FILE": "/run/credentials/forgejo-mcp/signing.pem",
			},
			wants: []string{"--cli"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := load(tc.args, tc.env)
			if err == nil {
				t.Fatal("an invalid resource-server configuration was accepted")
			}
			for _, w := range tc.wants {
				if !strings.Contains(err.Error(), w) {
					t.Fatalf("error %q does not mention %q", err, w)
				}
			}
		})
	}
}

func TestResourceServerNamesEveryMissingRequiredSettingInOneError(t *testing.T) {
	_, err := load([]string{"-t", "http", "-auth-mode", "resource-server"}, nil)
	if err == nil {
		t.Fatal("a resource-server configuration with nothing else was accepted")
	}
	for _, name := range []string{
		"-authorization-server", "-resource", "-forgejo-jwt-issuer", "-forgejo-jwt-signing-key-file",
	} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not name the missing %s", err, name)
		}
	}
}

func TestExplicitFlagEqualToTheDefaultStillBeatsTheEnvironment(t *testing.T) {
	// The precedence rows above depend on this: an explicit flag set to its
	// default must still count as passed.
	cfg := loadSettings(t, []string{"-host", "localhost"}, map[string]string{"FORGEJO_MCP_HOST": "0.0.0.0"})
	if cfg.Listen.Host != "localhost" {
		t.Fatalf("host = %q, an explicit -host localhost lost to the environment", cfg.Listen.Host)
	}
}
