// SPDX-License-Identifier: GPL-3.0-or-later

package forge

import (
	"fmt"
	"strconv"
	"strings"
)

// MajorVersion returns the major component of a Forgejo version such as
// "16.0.3" or "16.0.0-dev-741-6f391573+gitea-1.22.0".
//
// Only the major number counts. Semantic versioning would sort a development
// build of 16 below 16.0.0, but such a build already has every 16 feature this
// server relies on.
func MajorVersion(v string) (int, error) {
	s := strings.TrimPrefix(strings.TrimSpace(v), "v")
	major, _, _ := strings.Cut(s, ".")
	n, err := strconv.Atoi(major)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("the Forgejo version %q has no numeric major component", v)
	}
	return n, nil
}
