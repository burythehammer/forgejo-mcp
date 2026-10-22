// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"path/filepath"
	"strings"
	"testing"
)

// resourceServerEnv is a valid resource-server configuration, to which a test
// adds its own TLS settings.
func resourceServerEnv(extra ...string) func(string) string {
	base := []string{
		"FORGEJO_URL", "https://f.example.org",
		"FORGEJO_MCP_AUTH_MODE", "resource-server",
		"FORGEJO_MCP_AUTHORIZATION_SERVER", "https://sso.example.org",
		"FORGEJO_MCP_RESOURCE", "https://mcp.example.org/mcp",
		"FORGEJO_MCP_FORGEJO_JWT_ISSUER", "https://mcp.example.org/issuer",
		"FORGEJO_MCP_FORGEJO_JWT_SIGNING_KEY_FILE", "/k.pem",
	}
	return env(append(base, extra...)...)
}

var resourceServerArgs = []string{"-t", "http", "-host", "127.0.0.1", "-allowed-hosts", "mcp.example.org"}

// The provider's CA is loaded into the authorization-server settings only. The
// Forgejo TLS settings do not carry it, and skip-verify for Forgejo does not
// reach the provider.
func TestProviderCAIsSeparateFromForgejoTLS(t *testing.T) {
	t.Parallel()
	ca := writeFile(t, "idp-ca.pem", selfSignedPEM(t))
	cfg, _ := mustLoad(t, resourceServerArgs,
		resourceServerEnv("FORGEJO_MCP_AUTHORIZATION_SERVER_TLS_CA", ca, "FORGEJO_TLS_SKIP_VERIFY", "true"))

	if cfg.Auth.AuthorizationServerCAs == nil {
		t.Fatal("the provider CA was not loaded")
	}
	if cfg.Forgejo.TLS.CAPool != nil {
		t.Fatal("the provider CA leaked into the Forgejo TLS settings")
	}
	if !cfg.Forgejo.TLS.SkipVerify {
		t.Fatal("Forgejo skip-verify was not applied to Forgejo")
	}
}

func TestProviderCAFlagAndEnvironmentAgree(t *testing.T) {
	t.Parallel()
	ca := writeFile(t, "idp-ca.pem", selfSignedPEM(t))
	cfg, _ := mustLoad(t, append([]string{"-authorization-server-tls-ca", ca}, resourceServerArgs...), resourceServerEnv())
	if cfg.Auth.AuthorizationServerCAs == nil {
		t.Fatal("the -authorization-server-tls-ca flag did not load the CA")
	}
}

func TestProviderCAIsAbsentByDefault(t *testing.T) {
	t.Parallel()
	cfg, _ := mustLoad(t, resourceServerArgs, resourceServerEnv())
	if cfg.Auth.AuthorizationServerCAs != nil {
		t.Fatal("a provider CA was loaded without a setting")
	}
}

// An unreadable or empty provider CA file stops startup and names the setting.
func TestProviderCARejectsBadBundles(t *testing.T) {
	t.Parallel()
	empty := writeFile(t, "empty.pem", []byte("not a certificate\n"))
	missing := filepath.Join(t.TempDir(), "absent.pem")
	for name, path := range map[string]string{"unreadable": missing, "no certificates": empty} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, _, err := Load(resourceServerArgs, resourceServerEnv("FORGEJO_MCP_AUTHORIZATION_SERVER_TLS_CA", path), build)
			if err == nil || !strings.Contains(err.Error(), "authorization-server-tls-ca") {
				t.Fatalf("Load with %s provider CA: err = %v, want a refusal naming the setting", name, err)
			}
		})
	}
}

// The setting is a resource-server setting, so passthrough mode refuses it like
// the others.
func TestProviderCARefusedInPassthroughMode(t *testing.T) {
	t.Parallel()
	ca := writeFile(t, "idp-ca.pem", selfSignedPEM(t))
	_, _, err := Load([]string{"-t", "http", "-host", "127.0.0.1", "-allowed-hosts", "mcp.example.org"},
		env("FORGEJO_URL", "https://f.example.org", "FORGEJO_MCP_AUTHORIZATION_SERVER_TLS_CA", ca), build)
	if err == nil {
		t.Fatal("passthrough mode accepted a resource-server setting")
	}
}
