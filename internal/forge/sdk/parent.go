// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"errors"
	"net/http"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge"
)

// notFoundOr decides what a 404 from a list or membership endpoint means. Forgejo
// answers 404 both for a parent that does not exist and for a parent that has
// nothing to report, so the parent is probed on that path only. probe is the
// parent's own lookup error (nil when it exists). It returns nil when the parent
// exists, a not-found error carrying msg when it does not, and any other probe
// failure unchanged.
func notFoundOr(probe error, msg string) error {
	if forge.IsNotFound(probe) {
		return &forge.StatusError{StatusCode: http.StatusNotFound, Err: errors.New(msg)}
	}
	return probe
}

// orgNotFound is the message for a missing organisation. Forgejo also answers 404
// for an organisation the user cannot see, so the text says that too.
func orgNotFound(org string) string {
	return "organisation " + org + " not found (Forgejo also answers 404 for an organisation you cannot see)"
}
