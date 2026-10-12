// SPDX-License-Identifier: GPL-3.0-or-later

package issue

// NotFoundError is a failure Forgejo reported as "no such thing" (HTTP 404).
// The use cases test for it to word a delete of a missing label; its text is
// the underlying error's, so a message that does not test for it is unchanged.
type NotFoundError struct{ Err error }

func (e *NotFoundError) Error() string { return e.Err.Error() }
func (e *NotFoundError) Unwrap() error { return e.Err }
