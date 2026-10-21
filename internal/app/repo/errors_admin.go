// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import "errors"

// NotFoundError is a failure Forgejo reported as "no such thing" (HTTP 404) on
// a topic list. get_repo treats it as "no topics" rather than a failure. Its
// text is the underlying error's, so a message that does not test for it is
// unchanged.
type NotFoundError struct{ Err error }

func (e *NotFoundError) Error() string { return e.Err.Error() }
func (e *NotFoundError) Unwrap() error { return e.Err }

// errInvalidParams is the prefix of the commit-SHA validation errors. Its text
// is the one the tool has always returned.
var errInvalidParams = errors.New("invalid params")
