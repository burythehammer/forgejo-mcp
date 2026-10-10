// SPDX-License-Identifier: GPL-3.0-or-later

// Package fake is a stateful in-memory forge.Forgejo for tests.
//
// It behaves like a small Forgejo, not like a recorder: what a test writes
// through the port it reads back through the port, unknown things are 404
// (*forge.StatusError), and lists paginate. The forgetest contract suite holds
// the fake and the real adapter to the same behaviour, so a handler test that
// passes against the fake is passing against Forgejo's observable behaviour,
// not against an invention.
//
// A Forgejo is safe for concurrent use, so tests built on it run in parallel;
// each test makes its own with New.
//
// Layout: this file holds the aggregate and the factory. Each domain adds one
// file (pull.go, ...) with its state, its port methods and its seeding helpers,
// and one field on Forgejo.
package fake

import (
	"context"
	"net/http"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// Forgejo is the fake. Its zero value is not usable; call New.
type Forgejo struct {
	// Now supplies timestamps; replace it for deterministic times.
	Now func() time.Time

	pulls pullState
	admin repoAdminState
}

var _ forge.Forgejo = (*Forgejo)(nil)

// New returns an empty fake with no repositories.
func New() *Forgejo {
	return &Forgejo{Now: time.Now}
}

// Factory is a forge.Factory that hands out one fake, or fails with Err (for
// exercising the credential-policy path).
type Factory struct {
	Forgejo forge.Forgejo
	Err     error
}

var _ forge.Factory = Factory{}

// For implements forge.Factory.
func (f Factory) For(context.Context) (forge.Forgejo, error) {
	if f.Err != nil {
		return nil, f.Err
	}
	return f.Forgejo, nil
}

// defaultPageSize mirrors Forgejo's server default when a request names none.
const defaultPageSize = 30

// page applies Forgejo's pagination to n items and returns the [lo, hi) window.
// Page 0 is page 1, page -1 disables pagination (the SDK's convention), and an
// unset page size is the server default.
func page(opt forgejo_sdk.ListOptions, n int) (lo, hi int) {
	if opt.Page < 0 {
		return 0, n
	}
	size := opt.PageSize
	if size <= 0 {
		size = defaultPageSize
	}
	p := opt.Page
	if p == 0 {
		p = 1
	}
	lo = (p - 1) * size
	if lo > n {
		lo = n
	}
	hi = lo + size
	if hi > n {
		hi = n
	}
	return lo, hi
}

type errString string

func (e errString) Error() string { return string(e) }

func statusErr(code int, msg string) error {
	return &forge.StatusError{StatusCode: code, Err: errString(msg)}
}

func notFound(what string) error { return statusErr(http.StatusNotFound, what+" not found") }
