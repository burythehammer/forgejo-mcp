// SPDX-License-Identifier: GPL-3.0-or-later

package numarg

import (
	"fmt"
	"math"
)

// positiveBounds is the range of a positive whole number: one up to the
// largest int64.
var positiveBounds = Bounds{Min: 1, Max: math.MaxInt64}

// PositiveInt reads a positive whole number, for an ID or other count that
// must not be zero. given is false when raw is absent, null or empty. Any
// other value that is not a positive whole number is an error worded
// "<name> must be a positive integer", the wording the actions domain uses.
func PositiveInt(name string, raw any) (value int64, given bool, err error) {
	value, given, err = Int(name, raw, positiveBounds)
	if err != nil {
		return 0, false, fmt.Errorf("%s must be a positive integer", name)
	}
	return value, given, nil
}

// RequiredPositiveInt is PositiveInt for an argument that must be given. An
// absent, null or empty argument is an error worded "<name> is required".
func RequiredPositiveInt(name string, raw any) (int64, error) {
	value, given, err := PositiveInt(name, raw)
	if err != nil {
		return 0, err
	}
	if !given {
		return 0, fmt.Errorf("%s is required", name)
	}
	return value, nil
}
