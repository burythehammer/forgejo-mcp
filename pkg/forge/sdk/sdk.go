// SPDX-License-Identifier: GPL-3.0-or-later

// Package sdk is the real Forgejo adapter: it satisfies forge.Forgejo with the
// Forgejo SDK and, as domains migrate, the raw-HTTP workarounds behind one
// type, so callers cannot tell which transport serves a given method.
package sdk

import (
	"context"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/config"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forgejo"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// Adapter is the real forge.Forgejo, bound to one credential.
type Adapter struct {
	client *forgejo_sdk.Client
	raw    *forgejo.RawTarget
}

var _ forge.Forgejo = (*Adapter)(nil)

// Option configures an Adapter.
type Option func(*Adapter)

// WithRawHTTP binds the adapter's raw-HTTP calls (the endpoints the SDK lacks
// or cannot express) to a Forgejo instance and credential. Without it those
// methods fail; the factory always supplies it, and the contract tests point it
// at the stub server.
func WithRawHTTP(baseURL, token, userAgent string) Option {
	return func(a *Adapter) { a.raw = &forgejo.RawTarget{BaseURL: baseURL, Token: token, UserAgent: userAgent} }
}

// NewAdapter binds an adapter to an already-built SDK client. The factory uses
// it for every request; the contract tests use it to point the adapter at a
// stub server.
//
// Each domain's methods live in their own file in this package (pull.go, ...),
// all on *Adapter.
func NewAdapter(c *forgejo_sdk.Client, opts ...Option) *Adapter {
	a := &Adapter{client: c}
	for _, o := range opts {
		o(a)
	}
	return a
}

// statusErr wraps an SDK failure with the HTTP status Forgejo answered with,
// when there was one. The message is the SDK's, unchanged. ctx is not passed
// to the SDK by the adapter methods: the SDK client carries no per-call
// context, and the request deadline is the HTTP client's.
func statusErr(resp *forgejo_sdk.Response, err error) error {
	if err == nil || resp == nil || resp.Response == nil {
		return err
	}
	return &forge.StatusError{StatusCode: resp.StatusCode, Err: err}
}

// Factory is the real forge.Factory.
//
// The credential policy itself still lives in pkg/forgejo (credential.go),
// unchanged; For reaches it through the existing client construction, so a
// request is refused or served exactly as forgejo.Client(ctx) would.
type Factory struct {
	cfg config.Forgejo
}

var _ forge.Factory = (*Factory)(nil)

// NewFactory builds the factory for the Forgejo instance cfg describes.
func NewFactory(cfg config.Forgejo) *Factory { return &Factory{cfg: cfg} }

// BaseURL is the Forgejo instance the factory serves.
func (f *Factory) BaseURL() string { return f.cfg.URL }

// For implements forge.Factory.
func (f *Factory) For(ctx context.Context) (forge.Forgejo, error) {
	c, err := forgejo.Client(ctx)
	if err != nil {
		return nil, err
	}
	token, err := forgejo.TokenForRequest(ctx)
	if err != nil {
		return nil, err
	}
	return NewAdapter(c, WithRawHTTP(forgejo.GetBaseURL(), token, forgejo.UserAgent())), nil
}
