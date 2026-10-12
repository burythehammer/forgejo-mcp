// SPDX-License-Identifier: GPL-3.0-or-later

package forge

import (
	"errors"
	"fmt"
	"net/http"
)

// The domain error kinds every Forgejo adapter reports. The fake and the SDK
// adapter both produce them through StatusError, so a caller matches a kind
// with errors.Is and never reads an HTTP status. Renderers (tools and
// resources) switch on these; only the adapter layer maps statuses to kinds.
var (
	// ErrNotFound: Forgejo does not know the thing asked for (404).
	ErrNotFound = errors.New("not found")
	// ErrUnauthorized: the credential may not do this (401, 403).
	ErrUnauthorized = errors.New("unauthorized")
	// ErrValidation: Forgejo refused the input (400, 422).
	ErrValidation = errors.New("validation failed")
	// ErrUpstream: Forgejo failed on its side (5xx).
	ErrUpstream = errors.New("upstream failure")
)

// KindOf returns the domain error kind an HTTP status belongs to, or nil when
// the status has none (a conflict, for instance, is reported by its message).
func KindOf(code int) error {
	switch {
	case code == http.StatusUnauthorized, code == http.StatusForbidden:
		return ErrUnauthorized
	case code == http.StatusNotFound:
		return ErrNotFound
	case code == http.StatusBadRequest, code == http.StatusUnprocessableEntity:
		return ErrValidation
	case code >= 500 && code <= 599:
		return ErrUpstream
	}
	return nil
}

// StatusError is an error that carries the HTTP status Forgejo answered with.
// Both the real adapter and the fake return it for server-side failures. It
// matches the domain kind of its status with errors.Is.
//
// Error returns the underlying message unchanged, so wrapping a failure in a
// StatusError never alters what a tool reports.
type StatusError struct {
	StatusCode int
	Err        error
}

func (e *StatusError) Error() string { return e.Err.Error() }
func (e *StatusError) Unwrap() error { return e.Err }

// Is reports whether target is the domain kind of this error's status.
func (e *StatusError) Is(target error) bool {
	kind := KindOf(e.StatusCode)
	return kind != nil && target == kind
}

// StatusCode returns the HTTP status carried by err, or 0 when it has none.
func StatusCode(err error) int {
	var se *StatusError
	if errors.As(err, &se) {
		return se.StatusCode
	}
	return 0
}

// Detail is err's message with the HTTP status Forgejo answered with in front
// of it, when there was one: "403 list labels: forbidden". Renderers use it so
// the status stays visible to the agent without the renderer reading it.
func Detail(err error) string {
	if code := StatusCode(err); code != 0 {
		return fmt.Sprintf("%d %s", code, err.Error())
	}
	return err.Error()
}

// IsNotFound reports whether err is a Forgejo not-found.
func IsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }
