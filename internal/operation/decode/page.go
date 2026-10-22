// SPDX-License-Identifier: GPL-3.0-or-later

package decode

import "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/numarg"

// PageOr reads a page or limit argument as the list tools have always read it:
// an absent or zero number takes def. The zero test is made before the number
// is truncated, so 0.5 is not defaulted; it truncates to 0 and is passed on.
// The truncation is numarg.Lenient, the one unvalidated path in that module.
func PageOr(v float64, def int) int {
	if v == 0 {
		return def
	}
	return numarg.Lenient(v)
}

// IntOr is PageOr for the tools whose defaults were always applied to the
// truncated number, so 0.5 takes def.
func IntOr(v float64, def int) int {
	if n := numarg.Lenient(v); n != 0 {
		return n
	}
	return def
}
