// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/config"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/forgetest"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

func sdkClient(t *testing.T, url string) *forgejo_sdk.Client {
	t.Helper()
	c, err := forgejo_sdk.NewClient(url, forgejo_sdk.SetToken("sdk-token"))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// serve starts a server that answers the SDK probe and hands every other
// request to h.
func serve(t *testing.T, h http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/version" {
			forgetest.WriteVersion(w)
			return
		}
		h(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func rawAdapter(t *testing.T, srv *httptest.Server) *Adapter {
	t.Helper()
	c := sdkClient(t, srv.URL)
	return NewAdapter(c).WithRawBaseURL(srv.URL)
}

func TestListWikiPagesTotalCountHeader(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, header string
		want         int // -1: nil
	}{
		{"reported", "7", 7},
		{"absent", "", -1},
		{"unparsable", "not-a-number", -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := serve(t, func(w http.ResponseWriter, _ *http.Request) {
				if tc.header != "" {
					w.Header().Set("X-Total-Count", tc.header)
				}
				_, _ = w.Write([]byte(`[{"title":"One","sub_url":"One"}]`))
			})
			got, err := rawAdapter(t, srv).ListWikiPages(context.Background(), "o", "r", 1, 30)
			if err != nil {
				t.Fatal(err)
			}
			if tc.want < 0 && got.TotalCount != nil || tc.want >= 0 && (got.TotalCount == nil || *got.TotalCount != tc.want) {
				t.Fatalf("total count %v, want %d", got.TotalCount, tc.want)
			}
		})
	}
}

// Forgejo answers 404 for a repository without a wiki; the port reports that
// as an empty list, while a missing page's revisions stay an error.
func TestWikiNotFoundSemantics(t *testing.T) {
	t.Parallel()
	srv := serve(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) })
	a := rawAdapter(t, srv)
	list, err := a.ListWikiPages(context.Background(), "o", "r", 1, 30)
	if err != nil || list.Pages == nil || len(list.Pages) != 0 {
		t.Fatalf("list=%+v err=%v", list, err)
	}
	if _, err := a.GetWikiPageRevisions(context.Background(), "o", "r", "missing", 1, 30); !forge.IsNotFound(err) {
		t.Fatalf("revisions err=%v, want not found", err)
	}
}

func TestPackagePathsAreEscaped(t *testing.T) {
	t.Parallel()
	var escaped []string
	var mu sync.Mutex
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		escaped = append(escaped, r.Method+" "+r.URL.EscapedPath())
		mu.Unlock()
		_, _ = w.Write([]byte(`{}`))
	})
	a := rawAdapter(t, srv)
	ctx := context.Background()
	if _, err := a.GetPackage(ctx, "o/x", "container", "youscore/core", "1.0.0"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ListPackages(ctx, "o/x", forge.ListPackagesOptions{Page: 1, Limit: 1}); err == nil {
		t.Log("list of {} decodes as an object error; only the path matters here")
	}
	want := []string{"GET /api/v1/packages/o%2Fx/container/youscore%2Fcore/1.0.0", "GET /api/v1/packages/o%2Fx"}
	if strings.Join(escaped, "|") != strings.Join(want, "|") {
		t.Fatalf("paths %q, want %q", escaped, want)
	}
}

func TestRawCallsFollowTheFactoryCredential(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var auth []string
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		auth = append(auth, r.Header.Get("Authorization"))
		mu.Unlock()
		_, _ = w.Write([]byte(`[]`))
	})
	cfg := config.Forgejo{URL: srv.URL, Token: config.NewSecret(operatorValue)}

	f := NewFactory(cfg, "test")
	for _, ctx := range []context.Context{
		forge.WithToken(context.Background(), requestValue),
		forge.WithOperatorCredential(context.Background()),
	} {
		port, err := f.For(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := port.ListWikiPages(ctx, "o", "r", 1, 30); err != nil {
			t.Fatal(err)
		}
	}
	if len(auth) != 2 || auth[0] != "token "+requestValue || auth[1] != "token "+operatorValue {
		t.Fatalf("raw requests carried %q, want the request's token then the operator's", auth)
	}
}
