// SPDX-License-Identifier: GPL-3.0-or-later

package numarg

import (
	"math"
	"strconv"
)

// MaxPaging is the largest page or limit a list reads. A larger value is
// clamped to it, so no offset is computed from a value that could overflow and
// a huge page reads as an empty page rather than an error.
const MaxPaging = math.MaxInt32

// Paging reads a page or a limit argument as the list tools always have. An
// absent, null, empty, zero or negative value is 0, and the caller applies its
// own default for 0. A fraction is truncated, so 2.5 is 2 and 0.5 is 0. A value
// above MaxPaging is clamped to MaxPaging. A value that is not a number at all
// (a non-numeric string, a boolean, an object) is an error naming name.
func Paging(name string, raw any) (int, error) {
	var f float64
	switch x := raw.(type) {
	case nil:
		return 0, nil
	case float64:
		f = x
	case int:
		f = float64(x)
	case int64:
		f = float64(x)
	case string:
		if x == "" {
			return 0, nil
		}
		p, err := strconv.ParseFloat(x, 64)
		if err != nil {
			return 0, rangeError(name, Bounds{Min: 0, Max: MaxPaging})
		}
		f = p
	default:
		return 0, rangeError(name, Bounds{Min: 0, Max: MaxPaging})
	}
	switch {
	case math.IsNaN(f) || f < 1:
		return 0, nil
	case f >= MaxPaging:
		return MaxPaging, nil
	}
	return int(f), nil
}
