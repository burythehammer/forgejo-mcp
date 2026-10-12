// SPDX-License-Identifier: GPL-3.0-or-later

package decode

// PageOr reads a page or limit argument as the list tools have always read it:
// an absent or zero number takes def. The zero test is made before the number
// is truncated, so 0.5 is not defaulted; it truncates to 0 and is passed on.
func PageOr(v float64, def int) int {
	if v == 0 {
		return def
	}
	return int(v)
}

// IntOr is PageOr for the tools whose defaults were always applied to the
// truncated number, so 0.5 takes def.
func IntOr(v float64, def int) int {
	if n := int(v); n != 0 {
		return n
	}
	return def
}
