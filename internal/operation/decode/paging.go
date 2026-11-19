// SPDX-License-Identifier: GPL-3.0-or-later

package decode

import "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/numarg"

// PagingArgs reads the page and limit arguments of a list call through the
// numeric-argument module, so every list tool refuses the same bad value with
// the same message. An absent or zero page is page 1. An absent or zero limit
// is 0, which the list module reads as the tool's own default. Any other value
// must be a whole number up to numarg.MaxPaging.
func PagingArgs(page, limit float64) (int, int, error) {
	p, err := numarg.Paging("page", page)
	if err != nil {
		return 0, 0, err
	}
	l, err := numarg.Paging("limit", limit)
	if err != nil {
		return 0, 0, err
	}
	if p == 0 {
		p = 1
	}
	return p, l, nil
}
