// SPDX-License-Identifier: GPL-3.0-or-later

package to

import (
	"fmt"
	"math"
)

// BoundedInt reads an optional integer argument from a raw MCP call. An absent,
// nil or empty one takes defaultValue. Anything else must be a whole number
// within [minimum, maximum], or the error names the argument and its range.
func BoundedInt(args map[string]any, name string, defaultValue, minimum, maximum int) (int, error) {
	value, exists := args[name]
	if !exists || value == nil || value == "" {
		return defaultValue, nil
	}
	number, err := Float64(value)
	if err != nil || math.Trunc(number) != number || number < float64(minimum) || number > float64(maximum) {
		return 0, fmt.Errorf("%s must be an integer between %d and %d", name, minimum, maximum)
	}
	return int(number), nil
}
