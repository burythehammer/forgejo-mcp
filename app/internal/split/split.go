// SPDX-License-Identifier: GPL-3.0-or-later

// Package split holds the one comma-separated-list parser the use cases share.
package split

import "strings"

// CSV splits a comma-separated argument into its trimmed, non-empty items. It
// returns an empty (non-nil) slice for an empty or all-blank input, so a caller
// can tell "given, but names nothing" from "not given" by the slice being nil.
func CSV(s string) []string {
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
