// SPDX-License-Identifier: GPL-3.0-or-later

package transport

import (
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/config"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/sdk"
)

// newFactory mirrors operation.newFactory, which the composition root uses in
// production. It is repeated here because this package cannot import operation.
func newFactory(cfg config.Config, version string) *sdk.Factory {
	return sdk.NewFactory(cfg.Forgejo, version)
}
