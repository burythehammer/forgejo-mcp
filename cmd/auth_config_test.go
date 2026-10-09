// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"reflect"
	"strings"
	"testing"
)

// resourceServerSettingFlags pairs each resource-server-only setting's flag
// with its environment variable.
var resourceServerSettingFlags = []struct{ flagName, envVar string }{
	{"authorization-server", "FORGEJO_MCP_AUTHORIZATION_SERVER"},
	{"resource", "FORGEJO_MCP_RESOURCE"},
	{"resource-audience", "FORGEJO_MCP_RESOURCE_AUDIENCE"},
	{"scopes-supported", "FORGEJO_MCP_SCOPES_SUPPORTED"},
	{"forgejo-audience-claim", "FORGEJO_MCP_FORGEJO_AUDIENCE_CLAIM"},
	{"forgejo-jwt-issuer", "FORGEJO_MCP_FORGEJO_JWT_ISSUER"},
	{"forgejo-jwt-signing-key-file", "FORGEJO_MCP_FORGEJO_JWT_SIGNING_KEY_FILE"},
	{"forgejo-jwt-published-key-files", "FORGEJO_MCP_FORGEJO_JWT_PUBLISHED_KEY_FILES"},
}

// Spec oauth-resource-server, scenario "Issuer configured without the mode",
// for every resource-server-only setting, given as an environment variable and
// as a flag: passthrough mode records it and refuses to start, naming it.
func TestPassthroughRefusesEveryResourceServerSetting(t *testing.T) {
	for _, s := range resourceServerSettingFlags {
		t.Run(s.envVar, func(t *testing.T) {
			_, err := load(nil, map[string]string{s.envVar: "value"})
			if err == nil || !strings.Contains(err.Error(), s.envVar) || !strings.Contains(err.Error(), "requires -auth-mode resource-server") {
				t.Fatalf("passthrough with %s: error = %v, want a refusal naming it", s.envVar, err)
			}
		})
		t.Run("-"+s.flagName, func(t *testing.T) {
			_, err := load([]string{"-" + s.flagName, "value"}, nil)
			if err == nil || !strings.Contains(err.Error(), "-"+s.flagName) {
				t.Fatalf("passthrough with -%s: error = %v, want a refusal naming it", s.flagName, err)
			}
		})
	}
}

func TestAuthModeDefaultsToPassthroughWithNothingGiven(t *testing.T) {
	cfg := loadSettings(t, nil, nil)

	if cfg.Auth.Mode != "passthrough" {
		t.Fatalf("auth mode = %q, want passthrough", cfg.Auth.Mode)
	}
	if len(cfg.SuppliedResourceServerSettings()) != 0 {
		t.Fatalf("settings recorded as given: %v", cfg.SuppliedResourceServerSettings())
	}
	if cfg.Auth.ForgejoAudienceClaim != "forgejo_aud" {
		t.Fatalf("audience claim default = %q, want forgejo_aud", cfg.Auth.ForgejoAudienceClaim)
	}
}

// completeResourceServer is the settings Load requires of resource-server
// mode beyond those the tests below vary, so that a load is accepted and its
// resolved values can be read back.
var completeResourceServer = []string{
	"-t", "http", "-host", "127.0.0.1", "-auth-mode", "resource-server",
	"-forgejo-jwt-issuer", "https://localhost:8443/issuer",
	"-forgejo-jwt-signing-key-file", "signing.pem",
}

func TestResourceServerSettingsResolveFromFlagsAndEnvironment(t *testing.T) {
	cfg := loadSettings(t, append([]string{
		"-authorization-server", "https://sso.example.org",
		"-scopes-supported", "openid  profile email",
		"-forgejo-jwt-published-key-files", "next.pem, old.pem",
	}, completeResourceServer...), map[string]string{"FORGEJO_MCP_RESOURCE": "https://localhost:8443/mcp"})

	if cfg.Auth.Mode != "resource-server" {
		t.Fatalf("auth mode = %q", cfg.Auth.Mode)
	}
	if cfg.Auth.AuthorizationServer != "https://sso.example.org" || cfg.Auth.Resource != "https://localhost:8443/mcp" {
		t.Fatalf("authorization server %q, resource %q", cfg.Auth.AuthorizationServer, cfg.Auth.Resource)
	}
	if want := []string{"openid", "profile", "email"}; !reflect.DeepEqual(cfg.Auth.ScopesSupported, want) {
		t.Fatalf("scopes = %q, want %q", cfg.Auth.ScopesSupported, want)
	}
	if want := []string{"next.pem", "old.pem"}; !reflect.DeepEqual(cfg.Auth.ForgejoJWTPublishedKeyFiles, want) {
		t.Fatalf("published key files = %q, want %q", cfg.Auth.ForgejoJWTPublishedKeyFiles, want)
	}
	want := []string{
		"-authorization-server", "FORGEJO_MCP_RESOURCE", "-scopes-supported",
		"-forgejo-jwt-issuer", "-forgejo-jwt-signing-key-file", "-forgejo-jwt-published-key-files",
	}
	if got := cfg.SuppliedResourceServerSettings(); !reflect.DeepEqual(got, want) {
		t.Fatalf("settings recorded as given = %q, want %q", got, want)
	}
}

// A flag explicitly set to its default still wins over the environment, and
// still counts as given, so passthrough mode refuses it.
func TestExplicitAuthFlagEqualToTheDefaultBeatsTheEnvironment(t *testing.T) {
	_, err := load([]string{"-forgejo-audience-claim", "forgejo_aud"}, map[string]string{"FORGEJO_MCP_FORGEJO_AUDIENCE_CLAIM": "other_claim"})
	if err == nil || !strings.Contains(err.Error(), "-forgejo-audience-claim") || strings.Contains(err.Error(), "FORGEJO_MCP_FORGEJO_AUDIENCE_CLAIM") {
		t.Fatalf("passthrough error = %v, want a refusal naming only the flag", err)
	}

	cfg := loadSettings(t, append([]string{
		"-authorization-server", "https://sso.example.org", "-resource", "https://localhost:8443/mcp",
		"-forgejo-audience-claim", "forgejo_aud",
	}, completeResourceServer...), map[string]string{"FORGEJO_MCP_FORGEJO_AUDIENCE_CLAIM": "other_claim"})
	if cfg.Auth.ForgejoAudienceClaim != "forgejo_aud" {
		t.Fatalf("audience claim = %q, want the explicit flag value", cfg.Auth.ForgejoAudienceClaim)
	}
	for _, got := range cfg.SuppliedResourceServerSettings() {
		if got == "FORGEJO_MCP_FORGEJO_AUDIENCE_CLAIM" {
			t.Fatalf("the shadowed environment variable was recorded as given: %q", cfg.SuppliedResourceServerSettings())
		}
	}
}

// --cli mode never parses flags, so the environment alone must configure it.
func TestAuthSettingsResolveFromTheEnvironmentInCLIMode(t *testing.T) {
	_, err := load([]string{"--cli", "list"}, map[string]string{
		"FORGEJO_MCP_AUTH_MODE":          "resource-server",
		"FORGEJO_MCP_FORGEJO_JWT_ISSUER": "https://mcp.example.org/issuer",
	})
	if err == nil || !strings.Contains(err.Error(), "--cli") {
		t.Fatalf("error = %v, want the environment-configured resource-server mode refused under --cli", err)
	}
}
