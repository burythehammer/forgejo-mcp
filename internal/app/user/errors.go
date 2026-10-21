// SPDX-License-Identifier: GPL-3.0-or-later

package user

import "fmt"

// InvalidInputError is a time argument the use case could not parse. The
// message is What followed by the cause.
type InvalidInputError struct {
	What string
	Err  error
}

func (e *InvalidInputError) Error() string { return fmt.Sprintf("%s: %v", e.What, e.Err) }
func (e *InvalidInputError) Unwrap() error { return e.Err }
