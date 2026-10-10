// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import "git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"

// handlers serve the repo tools and resources. They hold the factory rather than
// a port so each call is bound to the credential of its own request.
type handlers struct{ forge forge.Factory }
