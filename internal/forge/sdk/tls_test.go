// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/config"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/log"
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

// tlsServer starts an httptest TLS server running h, with a self-signed
// certificate the way a private instance has one. It returns the server and a
// pool that trusts only that certificate: the bundle an operator would give.
func tlsServer(t *testing.T, h http.Handler) (*httptest.Server, *x509.CertPool) {
	t.Helper()
	srv := httptest.NewTLSServer(h)
	t.Cleanup(srv.Close)
	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	return srv, pool
}

// tlsForgejo serves a Forgejo-shaped API over TLS, with its bundle.
func tlsForgejo(t *testing.T) (*httptest.Server, *x509.CertPool) {
	t.Helper()
	return tlsServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/version" {
			_, _ = w.Write([]byte(`{"version":"12.0.0"}`))
			return
		}
		_, _ = w.Write([]byte(`[]`))
	}))
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
	srv, _ := tlsForgejo(t)
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
	srv, pool := tlsForgejo(t)
	cfg := config.Forgejo{
		URL:   srv.URL,
		Token: config.NewSecret("tok"),
		TLS:   config.TLS{CAPool: pool},
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
	srv, _ := tlsForgejo(t)
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

// A bundle and skip-verify set together both apply: the bundle is trusted and
// verification is off, so the connection succeeds on both paths.
func TestTLSSucceedsWithBundleAndSkipVerifyTogether(t *testing.T) {
	t.Parallel()
	srv, pool := tlsForgejo(t)
	cfg := config.Forgejo{
		URL:   srv.URL,
		Token: config.NewSecret("tok"),
		TLS:   config.TLS{CAPool: pool, SkipVerify: true},
	}
	sdkErr, rawErr := tlsCases(t, cfg)
	if sdkErr != nil {
		t.Errorf("SDK path with bundle and skip-verify: %v", sdkErr)
	}
	if rawErr != nil {
		t.Errorf("raw path with bundle and skip-verify: %v", rawErr)
	}
}

// The operator's path end to end: a PEM file in a temporary directory, loaded
// through config.Load with --tls-ca, builds the factory, and the factory's
// connection reaches a TLS instance whose certificate that file holds.
func TestTLSBundleFromFileThroughLoadReachesTheInstance(t *testing.T) {
	t.Parallel()
	srv, _ := tlsForgejo(t)
	bundle := filepath.Join(t.TempDir(), "forgejo-ca.pem")
	pemData := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	if err := os.WriteFile(bundle, pemData, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, _, err := config.Load(
		[]string{"--url", srv.URL, "--token", "tok", "--tls-ca", bundle},
		func(string) string { return "" },
		config.Build{Version: "test"},
	)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	sdkErr, rawErr := tlsCases(t, cfg.Forgejo)
	if sdkErr != nil {
		t.Errorf("SDK path with bundle from file: %v", sdkErr)
	}
	if rawErr != nil {
		t.Errorf("raw path with bundle from file: %v", rawErr)
	}
}
