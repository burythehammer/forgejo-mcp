// SPDX-License-Identifier: GPL-3.0-or-later

package numarg

import (
	"math"
	"strconv"
)

// Whole is Int for a number whose documented contract truncates a fraction
// toward zero, such as a length of time in seconds: 2.7 reads as 2. The
// truncated value must still be in range, so a value that would round down to
// nothing (0.5 with a minimum of 1) is refused like any other out-of-range
// value. Everything else is refused or read as Int reads it: a non-finite or
// overflowing number, a boolean and a malformed string are errors, and nil or
// an empty string is not given.
func Whole(name string, raw any, b Bounds) (value int64, given bool, err error) {
	if s, ok := raw.(string); ok && s != "" {
		p, perr := strconv.ParseFloat(s, 64)
		if perr != nil {
			return 0, false, rangeError(name, b)
		}
		raw = p
	}
	f, ok := raw.(float64)
	if !ok {
		return Int(name, raw, b)
	}
	if math.IsNaN(f) || math.IsInf(f, 0) || f < -0x1p63 || f >= 0x1p63 {
		return 0, false, rangeError(name, b)
	}
	return inRange(name, int64(f), b)
}
