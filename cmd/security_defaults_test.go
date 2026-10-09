// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	stdflag "flag"
	"os"
	"reflect"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation"
	flagPkg "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/flag"
)

// These tests pin how arguments and environment resolve into settings, and
// what the auth checks then refuse, by driving the same entry point the binary
// does (initFlags). They characterize behaviour that must survive the move to
// a typed Config, so they assert only outcomes: the resolved setting, or the
// refusal.

const (
	decoyToken = "decoy-operator-value-not-a-credential"
	testURL    = "https://forgejo.example.org"
)

// allConfigEnv is every environment variable initFlags consults.
func allConfigEnv() []string {
	env := []string{
		"FORGEJO_URL", "GITEA_HOST",
		"FORGEJO_ACCESS_TOKEN", "GITEA_ACCESS_TOKEN",
		"FORGEJO_USER_AGENT",
		"FORGEJO_DEBUG", "GITEA_DEBUG",
		"FORGEJO_MCP_HOST", "FORGEJO_MCP_ALLOWED_HOSTS", "FORGEJO_MCP_ALLOWED_ORIGINS",
		"FORGEJO_MCP_ALLOW_OPERATOR_TOKEN_FALLBACK",
		"FORGEJO_MCP_AUTH_MODE",
	}
	for _, s := range resourceServerSettings() {
		env = append(env, s.envVar)
	}
	return env
}

// loadSettings runs initFlags against args and env (on top of a cleared
// environment with only FORGEJO_URL set, unless env overrides it), and restores
// every piece of package state it touched.
func loadSettings(t *testing.T, args []string, env map[string]string) {
	t.Helper()

	strVars := []*string{
		&transport, &urlFlag, &token, &userAgent, &host, &allowedHosts, &allowedOrigins,
		&authMode, &authorizationServer, &resource, &resourceAudience, &scopesSupported,
		&forgejoAudienceClaim, &forgejoJWTIssuer, &forgejoJWTSigningKeyFile, &forgejoJWTPublishedKeyFiles,
	}
	savedStr := make([]string, len(strVars))
	for i, v := range strVars {
		savedStr[i] = *v
	}
	savedInts := [2]int{ssePort, httpPort}
	savedBools := [2]bool{allowFallback, debug}
	savedSet, savedArgs, savedCLI := flagSet, os.Args, cliMode
	savedPkg := struct {
		url, token, version, userAgent, host, mode, authServer, res, resAud, claim, issuer, keyFile string
		ssePort, httpPort                                                                           int
		hosts, origins, scopes, published, settings                                                 []string
		fallback, debug                                                                             bool
	}{
		flagPkg.URL, flagPkg.Token, flagPkg.Version, flagPkg.UserAgent, flagPkg.Host, flagPkg.AuthMode,
		flagPkg.AuthorizationServer, flagPkg.Resource, flagPkg.ResourceAudience, flagPkg.ForgejoAudienceClaim,
		flagPkg.ForgejoJWTIssuer, flagPkg.ForgejoJWTSigningKeyFile,
		flagPkg.SSEPort, flagPkg.HTTPPort,
		flagPkg.AllowedHosts, flagPkg.AllowedOrigins, flagPkg.ScopesSupported, flagPkg.ForgejoJWTPublishedKeyFiles,
		flagPkg.ResourceServerSettings,
		flagPkg.AllowOperatorTokenFallback, flagPkg.Debug,
	}
	t.Cleanup(func() {
		for i, v := range strVars {
			*v = savedStr[i]
		}
		ssePort, httpPort = savedInts[0], savedInts[1]
		allowFallback, debug = savedBools[0], savedBools[1]
		flagSet, os.Args, cliMode = savedSet, savedArgs, savedCLI
		flagPkg.URL, flagPkg.Token, flagPkg.Version, flagPkg.UserAgent, flagPkg.Host = savedPkg.url, savedPkg.token, savedPkg.version, savedPkg.userAgent, savedPkg.host
		flagPkg.AuthMode, flagPkg.AuthorizationServer, flagPkg.Resource = savedPkg.mode, savedPkg.authServer, savedPkg.res
		flagPkg.ResourceAudience, flagPkg.ForgejoAudienceClaim = savedPkg.resAud, savedPkg.claim
		flagPkg.ForgejoJWTIssuer, flagPkg.ForgejoJWTSigningKeyFile = savedPkg.issuer, savedPkg.keyFile
		flagPkg.SSEPort, flagPkg.HTTPPort = savedPkg.ssePort, savedPkg.httpPort
		flagPkg.AllowedHosts, flagPkg.AllowedOrigins = savedPkg.hosts, savedPkg.origins
		flagPkg.ScopesSupported, flagPkg.ForgejoJWTPublishedKeyFiles = savedPkg.scopes, savedPkg.published
		flagPkg.ResourceServerSettings = savedPkg.settings
		flagPkg.AllowOperatorTokenFallback, flagPkg.Debug = savedPkg.fallback, savedPkg.debug
	})

	for _, k := range allConfigEnv() {
		t.Setenv(k, "")
	}
	t.Setenv("FORGEJO_URL", testURL)
	for k, v := range env {
		t.Setenv(k, v)
	}
	cliMode = false
	os.Args = append([]string{"forgejo-mcp"}, args...)
	initFlags()
}

// validate runs the checks Execute runs after loading, on the loaded settings.
func validate(cli bool) error {
	return operation.ValidateAuthConfig(transport, cli)
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
	loadSettings(t, nil, nil)

	if transport != "stdio" {
		t.Errorf("transport = %q, want stdio", transport)
	}
	if flagPkg.Host != "localhost" {
		t.Errorf("host = %q, want the loopback name localhost", flagPkg.Host)
	}
	if len(flagPkg.AllowedHosts) != 0 {
		t.Errorf("allowed hosts = %q, want none", flagPkg.AllowedHosts)
	}
	if len(flagPkg.AllowedOrigins) != 0 {
		t.Errorf("allowed origins = %q, want none, which accepts no browser origin", flagPkg.AllowedOrigins)
	}
	if flagPkg.AllowOperatorTokenFallback {
		t.Error("the operator-token fallback is on by default")
	}
	if flagPkg.AuthMode != "passthrough" {
		t.Errorf("auth mode = %q, want passthrough", flagPkg.AuthMode)
	}
	if len(flagPkg.ResourceServerSettings) != 0 {
		t.Errorf("resource-server settings recorded as given: %q", flagPkg.ResourceServerSettings)
	}
	if flagPkg.SSEPort != 8080 || flagPkg.HTTPPort != 8080 {
		t.Errorf("ports = sse %d, http %d, want 8080 each", flagPkg.SSEPort, flagPkg.HTTPPort)
	}
	if flagPkg.Debug {
		t.Error("debug is on by default")
	}
	if err := validate(false); err != nil {
		t.Errorf("the default configuration is refused: %v", err)
	}
}

// Each row is one setting resolved three ways. flagArgs together with envVal
// must yield flagWant (the flag beats the environment); envVal alone must
// yield envWant; neither must yield defWant. A row with skipDefault has no
// valid "neither" state.
func TestFlagBeatsEnvironmentBeatsDefaultForEachSetting(t *testing.T) {
	rows := []struct {
		name        string
		flagArgs    []string
		envVar      string
		envVal      string
		get         func() any
		flagWant    any
		envWant     any
		defWant     any
		skipDefault bool
	}{
		{
			name: "url", flagArgs: []string{"-url", "https://flag.example.org"},
			envVar: "FORGEJO_URL", envVal: "https://env.example.org",
			get:      func() any { return flagPkg.URL },
			flagWant: "https://flag.example.org", envWant: "https://env.example.org", skipDefault: true,
		},
		{
			name: "token", flagArgs: []string{"-token", "flag-" + decoyToken},
			envVar: "FORGEJO_ACCESS_TOKEN", envVal: "env-" + decoyToken,
			get:      func() any { return flagPkg.Token },
			flagWant: "flag-" + decoyToken, envWant: "env-" + decoyToken, defWant: "",
		},
		{
			name: "user agent", flagArgs: []string{"-user-agent", "flag-agent"},
			envVar: "FORGEJO_USER_AGENT", envVal: "env-agent",
			get:      func() any { return flagPkg.UserAgent },
			flagWant: "flag-agent", envWant: "env-agent", defWant: "",
		},
		{
			name: "host", flagArgs: []string{"-host", "127.0.0.1"},
			envVar: "FORGEJO_MCP_HOST", envVal: "0.0.0.0",
			get:      func() any { return flagPkg.Host },
			flagWant: "127.0.0.1", envWant: "0.0.0.0", defWant: "localhost",
		},
		{
			name: "allowed hosts", flagArgs: []string{"-allowed-hosts", "flag.example.org"},
			envVar: "FORGEJO_MCP_ALLOWED_HOSTS", envVal: "env.example.org",
			get:      func() any { return flagPkg.AllowedHosts },
			flagWant: []string{"flag.example.org"}, envWant: []string{"env.example.org"}, defWant: []string(nil),
		},
		{
			name: "allowed origins", flagArgs: []string{"-allowed-origins", "https://flag.example.org"},
			envVar: "FORGEJO_MCP_ALLOWED_ORIGINS", envVal: "https://env.example.org",
			get:      func() any { return flagPkg.AllowedOrigins },
			flagWant: []string{"https://flag.example.org"}, envWant: []string{"https://env.example.org"}, defWant: []string(nil),
		},
		{
			// An explicit false must beat an environment variable asking for true.
			name: "operator token fallback", flagArgs: []string{"-allow-operator-token-fallback=false"},
			envVar: "FORGEJO_MCP_ALLOW_OPERATOR_TOKEN_FALLBACK", envVal: "true",
			get:      func() any { return flagPkg.AllowOperatorTokenFallback },
			flagWant: false, envWant: true, defWant: false,
		},
		{
			// An explicit passthrough must beat an environment variable asking for
			// resource-server.
			name: "auth mode", flagArgs: []string{"-auth-mode", "passthrough"},
			envVar: "FORGEJO_MCP_AUTH_MODE", envVal: "resource-server",
			get:      func() any { return flagPkg.AuthMode },
			flagWant: "passthrough", envWant: "resource-server", defWant: "passthrough",
		},
	}
	for _, r := range rows {
		t.Run(r.name+"/flag beats environment", func(t *testing.T) {
			loadSettings(t, r.flagArgs, map[string]string{r.envVar: r.envVal})
			if got := r.get(); !reflect.DeepEqual(got, r.flagWant) {
				t.Fatalf("got %#v, want the flag value %#v", got, r.flagWant)
			}
		})
		t.Run(r.name+"/environment beats default", func(t *testing.T) {
			loadSettings(t, nil, map[string]string{r.envVar: r.envVal})
			if got := r.get(); !reflect.DeepEqual(got, r.envWant) {
				t.Fatalf("got %#v, want the environment value %#v", got, r.envWant)
			}
		})
		if r.skipDefault {
			continue
		}
		t.Run(r.name+"/default", func(t *testing.T) {
			loadSettings(t, nil, nil)
			if got := r.get(); !reflect.DeepEqual(got, r.defWant) {
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
	loadSettings(t, []string{"-t", "sse", "-sse-port", "9090", "-http-port", "9091"}, nil)

	if transport != "sse" || flagPkg.SSEPort != 9090 || flagPkg.HTTPPort != 9091 {
		t.Fatalf("transport %q, sse port %d, http port %d", transport, flagPkg.SSEPort, flagPkg.HTTPPort)
	}
	loadSettings(t, nil, nil)
	if transport != "stdio" || flagPkg.SSEPort != 8080 || flagPkg.HTTPPort != 8080 {
		t.Fatalf("defaults moved: transport %q, sse port %d, http port %d", transport, flagPkg.SSEPort, flagPkg.HTTPPort)
	}
}

func TestDebugComesFromTheFlagOrTheEnvironment(t *testing.T) {
	loadSettings(t, []string{"-debug"}, nil)
	if !flagPkg.Debug {
		t.Error("-debug did not enable debug")
	}
	loadSettings(t, nil, map[string]string{"FORGEJO_DEBUG": "true"})
	if !flagPkg.Debug {
		t.Error("FORGEJO_DEBUG=true did not enable debug")
	}
	loadSettings(t, nil, map[string]string{"FORGEJO_DEBUG": "false"})
	if flagPkg.Debug {
		t.Error("FORGEJO_DEBUG=false enabled debug")
	}
}

func TestDeprecatedGiteaVariablesStillWorkAndLoseToForgejoOnes(t *testing.T) {
	loadSettings(t, nil, map[string]string{
		"FORGEJO_URL": "", "GITEA_HOST": "https://gitea.example.org",
		"GITEA_ACCESS_TOKEN": "gitea-" + decoyToken, "GITEA_DEBUG": "true",
	})
	if flagPkg.URL != "https://gitea.example.org" || flagPkg.Token != "gitea-"+decoyToken || !flagPkg.Debug {
		t.Fatalf("deprecated fallbacks lost: url %q, token set %t, debug %t",
			flagPkg.URL, flagPkg.Token != "", flagPkg.Debug)
	}

	loadSettings(t, nil, map[string]string{
		"FORGEJO_URL": "https://forgejo.example.org", "GITEA_HOST": "https://gitea.example.org",
		"FORGEJO_ACCESS_TOKEN": "forgejo-" + decoyToken, "GITEA_ACCESS_TOKEN": "gitea-" + decoyToken,
	})
	if flagPkg.URL != "https://forgejo.example.org" || flagPkg.Token != "forgejo-"+decoyToken {
		t.Fatalf("a deprecated variable beat its replacement: url %q", flagPkg.URL)
	}
}

// Passthrough refusing resource-server-only settings is covered per setting in
// auth_config_test.go; this proves the same refusal through the full load, for
// a setting given by flag and by environment variable, on every transport.
func TestPassthroughRefusesAResourceServerSettingOnEveryTransport(t *testing.T) {
	for _, tr := range []string{"stdio", "sse", "http"} {
		t.Run(tr+"/flag", func(t *testing.T) {
			loadSettings(t, []string{"-t", tr, "-forgejo-jwt-issuer", "https://mcp.example.org/issuer"}, nil)
			err := validate(false)
			if err == nil || !strings.Contains(err.Error(), "-forgejo-jwt-issuer") {
				t.Fatalf("error = %v, want a refusal naming the flag", err)
			}
		})
		t.Run(tr+"/environment", func(t *testing.T) {
			loadSettings(t, []string{"-t", tr}, map[string]string{"FORGEJO_MCP_RESOURCE": "https://mcp.example.org/mcp"})
			err := validate(false)
			if err == nil || !strings.Contains(err.Error(), "FORGEJO_MCP_RESOURCE") {
				t.Fatalf("error = %v, want a refusal naming the variable", err)
			}
		})
	}
}

func TestResourceServerAcceptsACompleteConfiguration(t *testing.T) {
	// The control for the refusals below: each of them changes one thing in
	// this configuration.
	loadSettings(t, resourceServerArgs(), nil)
	if err := validate(false); err != nil {
		t.Fatalf("a complete resource-server configuration was refused: %v", err)
	}
}

func TestResourceServerRefusals(t *testing.T) {
	cases := []struct {
		name  string
		args  []string
		env   map[string]string
		cli   bool
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
			name: "cli mode", args: resourceServerArgs(), cli: true,
			wants: []string{"--cli"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			loadSettings(t, tc.args, tc.env)
			err := validate(tc.cli)
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
	loadSettings(t, []string{"-t", "http", "-auth-mode", "resource-server"}, nil)
	err := validate(false)
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

func TestExplicitFlagSetStaysDistinguishableAfterALoad(t *testing.T) {
	// The precedence rows above depend on this: an explicit flag set to its
	// default must still count as passed.
	loadSettings(t, []string{"-host", "localhost"}, map[string]string{"FORGEJO_MCP_HOST": "0.0.0.0"})
	if flagPkg.Host != "localhost" {
		t.Fatalf("host = %q, an explicit -host localhost lost to the environment", flagPkg.Host)
	}
	var passed []string
	flagSet.Visit(func(f *stdflag.Flag) { passed = append(passed, f.Name) })
	if !reflect.DeepEqual(passed, []string{"host"}) {
		t.Fatalf("flags reported as passed = %q, want only host", passed)
	}
}
