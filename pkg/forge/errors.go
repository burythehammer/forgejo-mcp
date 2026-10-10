// SPDX-License-Identifier: GPL-3.0-or-later

package forge

import (
	"errors"
	"net/http"
)

// StatusError is an error that carries the HTTP status Forgejo answered with.
// Both the real adapter and the fake return it for server-side failures, so a
// caller can tell "not found" from "forbidden" without parsing messages.
//
// Error returns the underlying message unchanged, so wrapping a failure in a
// StatusError never alters what a tool reports.
type StatusError struct {
	StatusCode int
	Err        error
}

func (e *StatusError) Error() string { return e.Err.Error() }
func (e *StatusError) Unwrap() error { return e.Err }

// StatusCode returns the HTTP status carried by err, or 0 when it has none.
func StatusCode(err error) int {
	var se *StatusError
	if errors.As(err, &se) {
		return se.StatusCode
	}
	return 0
}

// IsNotFound reports whether err is a 404 from Forgejo.
func IsNotFound(err error) bool { return StatusCode(err) == http.StatusNotFound }
