// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"

// handlers serve the repo tools; they hold the factory so each call uses its own request's credential.
type handlers struct{ forge forge.Factory }
