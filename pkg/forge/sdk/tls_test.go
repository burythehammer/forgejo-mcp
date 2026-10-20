// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/config"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// TestSkipVerifyWarnsOnceAtConstruction is not parallel: it swaps the package
// logger.
func TestSkipVerifyWarnsOnceAtConstruction(t *testing.T) {
	prev := log.Logger()
	core, logs := observer.New(zap.WarnLevel)
	log.SetDefault(zap.New(core))
	t.Cleanup(func() { log.SetDefault(prev) })

	const marker = "TLS certificate verification is OFF"
	NewFactory(config.Forgejo{URL: "https://f.example.org", TLS: config.TLS{SkipVerify: true}}, "test")
	if n := logs.FilterMessageSnippet(marker).Len(); n != 1 {
		t.Fatalf("skip-verify warnings = %d, want 1", n)
	}
	NewFactory(config.Forgejo{URL: "https://f.example.org"}, "test")
	if n := logs.FilterMessageSnippet(marker).Len(); n != 1 {
		t.Fatalf("default config logged a skip-verify warning; total = %d", n)
	}
}

// tlsForgejo serves a Forgejo-shaped API over TLS with a self-signed
// certificate, the way a private instance does.
func tlsForgejo(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/version" {
			_, _ = w.Write([]byte(`{"version":"12.0.0"}`))
			return
		}
		_, _ = w.Write([]byte(`[]`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// tlsCases runs the SDK path (the connection check, which builds the SDK
// client) and the raw-HTTP path (ProbeServerVersion, which uses the factory's
// raw client directly and builds no SDK client) and reports each outcome.
func tlsCases(t *testing.T, cfg config.Forgejo) (sdkErr, rawErr error) {
	t.Helper()
	f := NewFactory(cfg, "test")
	sdkErr = f.VerifyConnection(context.Background())
	_, rawErr = f.ProbeServerVersion(context.Background())
	return sdkErr, rawErr
}

func TestTLSFailsAgainstUntrustedCertificateByDefault(t *testing.T) {
	t.Parallel()
	srv := tlsForgejo(t)
	sdkErr, rawErr := tlsCases(t, config.Forgejo{URL: srv.URL, Token: config.NewSecret("tok")})
	if sdkErr == nil {
		t.Errorf("SDK path verified an untrusted certificate")
	}
	if rawErr == nil {
		t.Errorf("raw path verified an untrusted certificate")
	}
}

func TestTLSSucceedsWhenTheInstanceCAIsInTheBundle(t *testing.T) {
	t.Parallel()
	srv := tlsForgejo(t)
	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	cfg := config.Forgejo{
		URL:   srv.URL,
		Token: config.NewSecret("tok"),
		TLS:   config.TLS{CAPool: pool, CAFile: "ca.pem"},
	}
	sdkErr, rawErr := tlsCases(t, cfg)
	if sdkErr != nil {
		t.Errorf("SDK path with CA in bundle: %v", sdkErr)
	}
	if rawErr != nil {
		t.Errorf("raw path with CA in bundle: %v", rawErr)
	}
}

func TestTLSSucceedsWithSkipVerify(t *testing.T) {
	t.Parallel()
	srv := tlsForgejo(t)
	cfg := config.Forgejo{
		URL:   srv.URL,
		Token: config.NewSecret("tok"),
		TLS:   config.TLS{SkipVerify: true},
	}
	sdkErr, rawErr := tlsCases(t, cfg)
	if sdkErr != nil {
		t.Errorf("SDK path with skip-verify: %v", sdkErr)
	}
	if rawErr != nil {
		t.Errorf("raw path with skip-verify: %v", rawErr)
	}
}
