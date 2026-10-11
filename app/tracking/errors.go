// SPDX-License-Identifier: GPL-3.0-or-later

package tracking

import "fmt"

// InvalidInputError is an argument the use case could not make sense of, such
// as a since bound that is not RFC 3339 or a duration that does not parse. The
// message is What followed by the cause.
type InvalidInputError struct {
	What string
	Err  error
}

func (e *InvalidInputError) Error() string { return fmt.Sprintf("%s: %v", e.What, e.Err) }
func (e *InvalidInputError) Unwrap() error { return e.Err }
