// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"fmt"
	"net/http"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/forge"
)

// notFoundOr decides what a 404 from a list or membership endpoint means. Forgejo
// answers 404 both for a parent that does not exist and for a parent that has
// nothing to report, so the parent is probed on that path only. probe is the
// parent's own lookup error (nil when it exists). It returns nil when the parent
// exists, a not-found error naming it when it does not, and any other probe
// failure unchanged.
func notFoundOr(probe error, what string) error {
	if forge.IsNotFound(probe) {
		return &forge.StatusError{StatusCode: http.StatusNotFound, Err: fmt.Errorf("%s not found", what)}
	}
	return probe
}
