// SPDX-License-Identifier: GPL-3.0-or-later

package oauthrs

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwk"
)

// throwawayCA is a private certificate authority with one server certificate
// for 127.0.0.1, the shape of an identity provider behind a private CA.
type throwawayCA struct {
	pool   *x509.CertPool
	server tls.Certificate
}

func newThrowawayCA(t *testing.T) throwawayCA {
	t.Helper()
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "throwaway provider CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, caCert, &leafKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	leafPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER})
	keyDER, err := x509.MarshalPKCS8PrivateKey(leafKey)
	if err != nil {
		t.Fatal(err)
	}
	server, err := tls.X509KeyPair(leafPEM, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))
	if err != nil {
		t.Fatal(err)
	}

	pool := x509.NewCertPool()
	pool.AddCert(caCert)
	return throwawayCA{pool: pool, server: server}
}

// newTLSProvider serves discovery and a key set over TLS with a certificate
// from ca. Its issuer is its own https URL.
func newTLSProvider(t *testing.T, ca throwawayCA) *httptest.Server {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := jwk.Import(key.Public())
	if err != nil {
		t.Fatal(err)
	}
	_ = pub.Set(jwk.KeyIDKey, "tls-1")
	_ = pub.Set(jwk.KeyUsageKey, jwk.ForSignature)
	set := jwk.NewSet()
	_ = set.AddKey(pub)

	var srv *httptest.Server
	srv = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{"issuer": srv.URL, "jwks_uri": srv.URL + "/jwks"})
		case "/jwks":
			_ = json.NewEncoder(w).Encode(set)
		default:
			http.NotFound(w, r)
		}
	}))
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{ca.server}}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return srv
}

// providerClient returns the client an operator's CA setting produces:
// nil roots mean the system roots only.
func providerClient(pool *x509.CertPool) *http.Client {
	var rt http.RoundTripper
	if pool != nil {
		rt = &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool}}
	}
	return NewHTTPClient(rt)
}

// The provider's discovery and key fetches succeed only when its private CA
// is trusted, and a different CA does not satisfy them.
func TestProviderLookupsTrustOnlyTheConfiguredCA(t *testing.T) {
	ca := newThrowawayCA(t)
	other := newThrowawayCA(t)
	srv := newTLSProvider(t, ca)
	ctx := context.Background()

	if _, err := DiscoverProvider(ctx, providerClient(nil), srv.URL); err == nil {
		t.Fatal("DiscoverProvider trusted a private CA that was not configured")
	}
	if _, err := NewKeySet(ctx, srv.URL+"/jwks", KeySetOptions{Client: providerClient(nil)}); err == nil {
		t.Fatal("NewKeySet trusted a private CA that was not configured")
	}
	if _, err := DiscoverProvider(ctx, providerClient(other.pool), srv.URL); err == nil {
		t.Fatal("DiscoverProvider accepted a certificate from a different CA")
	}

	m, err := DiscoverProvider(ctx, providerClient(ca.pool), srv.URL)
	if err != nil {
		t.Fatalf("DiscoverProvider with the configured CA: %v", err)
	}
	if _, err := NewKeySet(ctx, m.JWKSURI, KeySetOptions{Client: providerClient(ca.pool)}); err != nil {
		t.Fatalf("NewKeySet with the configured CA: %v", err)
	}
}
