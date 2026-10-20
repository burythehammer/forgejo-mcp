// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/config"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/forge"
)

// tlsEveryPath serves a TLS instance that answers every raw-HTTP path the
// adapter uses: the version probe, a JSON POST, a text job log, a same-origin
// attachment download and a multipart attachment upload.
func tlsEveryPath(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/version":
			_, _ = w.Write([]byte(`{"version":"12.0.0"}`))
		case strings.HasSuffix(r.URL.Path, "/logs"):
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("log"))
		case strings.HasPrefix(r.URL.Path, "/attachments/"):
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write([]byte("bytes"))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/assets"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":1,"name":"f.txt"}`))
		default:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// rawPathCalls is one call per raw-HTTP mechanism in the adapter, made through
// the factory's adapter for a request that carries its own token (the
// production wiring, built by Factory.For).
var rawPathCalls = []struct {
	name string
	call func(ctx context.Context, p forge.Forgejo, base string) error
}{
	{"json", func(ctx context.Context, p forge.Forgejo, _ string) error {
		return p.CancelWorkflowRun(ctx, "o", "r", 1)
	}},
	{"api raw", func(ctx context.Context, p forge.Forgejo, _ string) error {
		_, err := p.GetJobLog(ctx, "o", "r", 1, 0, "", 1024)
		return err
	}},
	{"download", func(ctx context.Context, p forge.Forgejo, base string) error {
		_, err := p.DownloadAttachment(ctx, base+"/attachments/a1")
		return err
	}},
	{"multipart", func(ctx context.Context, p forge.Forgejo, _ string) error {
		_, err := p.CreateIssueAttachment(ctx, "o", "r", 1, forge.Upload{
			Filename: "f.txt",
			MIMEType: "text/plain",
			Content:  strings.NewReader("hello"),
		})
		return err
	}},
}

// TestTLSEveryRawPathHonoursTheConfig runs each raw-HTTP mechanism against a
// self-signed instance: it fails by default, succeeds when the instance's CA is
// in the bundle, and succeeds with skip-verify.
func TestTLSEveryRawPathHonoursTheConfig(t *testing.T) {
	t.Parallel()
	for _, pc := range rawPathCalls {
		t.Run(pc.name, func(t *testing.T) {
			t.Parallel()
			srv := tlsEveryPath(t)

			pool := x509.NewCertPool()
			pool.AddCert(srv.Certificate())

			cases := []struct {
				name    string
				tls     config.TLS
				wantErr bool
			}{
				{"default refuses untrusted certificate", config.TLS{}, true},
				{"CA in bundle succeeds", config.TLS{CAPool: pool, CAFile: "ca.pem"}, false},
				{"skip-verify succeeds", config.TLS{SkipVerify: true}, false},
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					f := NewFactory(config.Forgejo{URL: srv.URL, TLS: tc.tls}, "test")
					// Record the version so building the adapter sends no
					// request: the call below is then the only request made.
					f.serverVersion.Store("12.0.0")
					ctx := forge.WithToken(context.Background(), "tok")
					p, err := f.For(ctx)
					if err != nil {
						t.Fatalf("For: %v", err)
					}
					err = pc.call(ctx, p, srv.URL)
					if tc.wantErr && err == nil {
						t.Fatalf("%s verified an untrusted certificate", pc.name)
					}
					if !tc.wantErr && err != nil {
						t.Fatalf("%s: %v", pc.name, err)
					}
				})
			}
		})
	}
}
