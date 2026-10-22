// SPDX-License-Identifier: GPL-3.0-or-later

package hook

import (
	apphook "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/hook"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/numarg"
)

// refID reads a hook ID with the numeric-argument module. Any value that is not
// a positive whole number, including a missing one, is refused with the use
// case's ErrRequired, so the wording and the no-request behaviour stay as pinned.
func refID(raw any) (int64, error) {
	id, err := numarg.PositiveID("id", raw)
	if err != nil {
		return 0, apphook.ErrRequired
	}
	return id, nil
}
