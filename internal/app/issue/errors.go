// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import "fmt"

// NotFoundError is a failure Forgejo reported as "no such thing" (HTTP 404).
// The use cases test for it to word a delete of a missing label; its text is
// the underlying error's, so a message that does not test for it is unchanged.
type NotFoundError struct{ Err error }

func (e *NotFoundError) Error() string { return e.Err.Error() }
func (e *NotFoundError) Unwrap() error { return e.Err }

// InaccessibleError is a failure Forgejo reported as "this token cannot see
// that" (HTTP 401, 403 or 404) on a best-effort scan. The scan skips it; any
// other failure ends the scan.
type InaccessibleError struct{ Err error }

func (e *InaccessibleError) Error() string { return e.Err.Error() }
func (e *InaccessibleError) Unwrap() error { return e.Err }

// ExclusiveConflictError is a label request that names two labels from one
// exclusive scope. Forgejo would keep the first and drop the rest while still
// answering success, so the request is refused before any write. It is an
// invalid argument, not a Forgejo failure.
type ExclusiveConflictError struct {
	Scope       string
	First, Next string
}

func (e *ExclusiveConflictError) Error() string {
	return fmt.Sprintf(
		"labels %q and %q are both from exclusive scope %q, and an issue carries at most one label from it; pass one of them",
		e.First, e.Next, e.Scope,
	)
}
