// SPDX-License-Identifier: GPL-3.0-or-later

package forgetest

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/fake"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// Stub is the one stub HTTP server that stands in for Forgejo wherever a test
// needs the real adapter's wire: the contract suite, and any adapter test. It
// answers the SDK's startup probe (the version request every client makes when
// it is built) in exactly one place, WriteVersion, and serves the REST
// endpoints of each migrated domain from a Fake.
//
// The Fake is the stub's state, and doubles as its seeding interface, so a test
// seeds through Stub.Fake and reads back through the adapter. The stub is a
// translation layer, not a second implementation: it owns no behaviour that the
// fake does not, so the contract suite compares the adapter's request and
// response mapping against the fake's semantics.
type Stub struct {
	*httptest.Server
	// Fake holds the stub's state.
	Fake *fake.Forgejo
}

// NewStub starts a stub server, closed when the test ends.
func NewStub(t testing.TB) *Stub {
	t.Helper()
	s := &Stub{Fake: fake.New()}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/version", func(w http.ResponseWriter, _ *http.Request) { WriteVersion(w) })
	s.mountPulls(mux)
	s.mountOrgs(mux)
	s.mountUsers(mux)
	s.mountRepoAdmin(mux)
	s.Server = httptest.NewServer(mux)
	t.Cleanup(s.Close)
	return s
}

// NewClient builds an SDK client for the stub, running the startup probe.
func (s *Stub) NewClient(t testing.TB) *forgejo_sdk.Client {
	t.Helper()
	c, err := forgejo_sdk.NewClient(s.URL, forgejo_sdk.SetToken("stub-token"))
	if err != nil {
		t.Fatalf("building SDK client against stub: %v", err)
	}
	return c
}

// WriteVersion answers the SDK's startup probe. Any test that stands up its own
// server for the SDK should call it rather than re-stub the probe.
func WriteVersion(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"version":"11.0.0"}`))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

// writeErr renders a port error the way Forgejo does: its status, a JSON
// message. Errors without a status are validation failures (422).
func writeErr(w http.ResponseWriter, err error) {
	code := forge.StatusCode(err)
	if code == 0 {
		code = http.StatusUnprocessableEntity
	}
	writeJSON(w, code, map[string]string{"message": err.Error()})
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeErr(w, errors.New("bad request body: "+err.Error()))
		return false
	}
	return true
}

func listOptions(r *http.Request) forgejo_sdk.ListOptions {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	size, _ := strconv.Atoi(q.Get("limit"))
	return forgejo_sdk.ListOptions{Page: page, PageSize: size}
}

func pathIndex(r *http.Request, name string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSuffix(r.PathValue(name), ".diff"), 10, 64)
	return n
}
