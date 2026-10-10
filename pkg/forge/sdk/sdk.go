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
}

var _ forge.Forgejo = (*Adapter)(nil)

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
	return &Adapter{client: c}, nil
}
