// SPDX-License-Identifier: GPL-3.0-or-later

// Package numarg turns a raw JSON number argument into a validated whole
// number. It is the one home of the rule "an integer, in range, with no
// fraction and no overflow". It is MCP-free and SDK-free, so a use case may
// import it.
//
// The raw value is what the MCP layer hands over after JSON decoding: float64
// for a number, string for a string, nil for an argument that was not sent or
// was sent as null. Integer and int64 are accepted too, for callers that build
// arguments in Go.
package numarg

import (
	"fmt"
	"math"
	"strconv"
)

// Bounds is the range an argument may take. Zero is allowed exactly when
// Min is zero or less: a positive ID uses Min 1.
type Bounds struct {
	Min, Max int64
}

// Int converts raw into a whole number within b. given is false when raw is
// nil, a null, or an empty string: the caller applies its default. Any other
// value that is not a whole number in range is an error naming name and the
// range, for example "limit must be an integer between 1 and 50".
func Int(name string, raw any, b Bounds) (value int64, given bool, err error) {
	var f float64
	switch x := raw.(type) {
	case nil:
		return 0, false, nil
	case float64:
		f = x
	case int:
		return inRange(name, int64(x), b)
	case int64:
		return inRange(name, x, b)
	case string:
		if x == "" {
			return 0, false, nil
		}
		p, perr := strconv.ParseFloat(x, 64)
		if perr != nil {
			return 0, false, rangeError(name, b)
		}
		f = p
	default:
		return 0, false, rangeError(name, b)
	}
	if math.IsNaN(f) || math.IsInf(f, 0) || math.Trunc(f) != f || f < -0x1p63 || f >= 0x1p63 {
		return 0, false, rangeError(name, b)
	}
	return inRange(name, int64(f), b)
}

// RequiredInt is Int for an argument that must be given. An absent, null or
// empty argument is an error worded "<name> is required".
func RequiredInt(name string, raw any, b Bounds) (int64, error) {
	value, given, err := Int(name, raw, b)
	if err != nil {
		return 0, err
	}
	if !given {
		return 0, fmt.Errorf("%s is required", name)
	}
	return value, nil
}

// PositiveID is RequiredInt for an ID, index or count that must name something:
// a whole number from 1 up to the largest int64. Zero, negatives, fractions and
// overflow are refused with the same wording as any other integer argument.
func PositiveID(name string, raw any) (int64, error) {
	return RequiredInt(name, raw, Bounds{Min: 1, Max: math.MaxInt64})
}

// Lenient truncates v to a whole number without checking it. It is the legacy
// path that the page and limit helpers in the decode package keep, because
// their pinned behaviour is to pass a fractional page on as its truncation.
// New code uses Int.
func Lenient(v float64) int {
	return int(v)
}

func inRange(name string, n int64, b Bounds) (int64, bool, error) {
	if n < b.Min || n > b.Max {
		return 0, false, rangeError(name, b)
	}
	return n, true, nil
}

func rangeError(name string, b Bounds) error {
	return fmt.Errorf("%s must be an integer between %d and %d", name, b.Min, b.Max)
}
