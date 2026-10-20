// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// selfSignedPEM returns a PEM-encoded self-signed certificate, the same shape a
// private CA bundle takes.
func selfSignedPEM(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "private test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func writeFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadTLSDefaultsToVerifyingWithNoBundle(t *testing.T) {
	t.Parallel()
	cfg, _ := mustLoad(t, nil, env("FORGEJO_URL", "https://f.example.org"))
	if cfg.Forgejo.TLS.CAFile != "" || cfg.Forgejo.TLS.CAPool != nil || cfg.Forgejo.TLS.SkipVerify {
		t.Fatalf("TLS = %+v, want zero value", cfg.Forgejo.TLS)
	}
}

func TestLoadTLSPrecedence(t *testing.T) {
	t.Parallel()
	bundle := writeFile(t, "ca.pem", selfSignedPEM(t))
	other := writeFile(t, "other.pem", selfSignedPEM(t))
	base := []string{"FORGEJO_URL", "https://f.example.org"}

	cases := []struct {
		name     string
		args     []string
		env      []string
		wantFile string
		wantSkip bool
	}{
		{name: "ca flag", args: []string{"-tls-ca", bundle}, wantFile: bundle},
		{name: "ca env", env: []string{"FORGEJO_TLS_CA", bundle}, wantFile: bundle},
		{name: "ca flag beats env", args: []string{"-tls-ca", bundle},
			env: []string{"FORGEJO_TLS_CA", other}, wantFile: bundle},
		{name: "skip flag", args: []string{"-tls-skip-verify"}, wantSkip: true},
		{name: "skip env", env: []string{"FORGEJO_TLS_SKIP_VERIFY", "true"}, wantSkip: true},
		{name: "skip flag beats env false", args: []string{"-tls-skip-verify"},
			env: []string{"FORGEJO_TLS_SKIP_VERIFY", "false"}, wantSkip: true},
		{name: "skip env false is off", env: []string{"FORGEJO_TLS_SKIP_VERIFY", "false"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := append(append([]string{}, base...), tc.env...)
			cfg, _ := mustLoad(t, tc.args, env(e...))
			got := cfg.Forgejo.TLS
			if got.CAFile != tc.wantFile {
				t.Errorf("CAFile = %q, want %q", got.CAFile, tc.wantFile)
			}
			if got.SkipVerify != tc.wantSkip {
				t.Errorf("SkipVerify = %v, want %v", got.SkipVerify, tc.wantSkip)
			}
			if tc.wantFile != "" && got.CAPool == nil {
				t.Errorf("CAPool is nil although a bundle was given")
			}
		})
	}
}

func TestLoadTLSRejectsBadBundles(t *testing.T) {
	t.Parallel()
	empty := writeFile(t, "empty.pem", []byte("not a certificate\n"))
	missing := filepath.Join(t.TempDir(), "absent.pem")
	base := env("FORGEJO_URL", "https://f.example.org")

	for name, path := range map[string]string{"unreadable": missing, "no certificates": empty} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, _, err := Load([]string{"-tls-ca", path}, base, build)
			if err == nil {
				t.Fatalf("Load accepted %s bundle %q", name, path)
			}
		})
	}
}
