// SPDX-License-Identifier: GPL-3.0-or-later

// Package lines holds the line-range slicing shared by the file and wiki
// tools. It is pure, so use cases under app/ may import it.
package lines

import (
	"fmt"
	"strings"
)

// Slice returns the 1-indexed inclusive [start, end] line range of content.
// start=0 means "from line 1"; end=0 means "to the last line". Out-of-range
// values clamp; an inverted range after clamping is reported as an error.
func Slice(content string, start, end int) (string, error) {
	lines := strings.Split(content, "\n")
	count := len(lines)

	if start <= 0 {
		start = 1
	}
	if end <= 0 {
		end = count
	}
	if end > count {
		end = count
	}
	if start > end {
		return "", fmt.Errorf("start_line (%d) is after end_line (%d) for a file with %d lines", start, end, count)
	}

	return strings.Join(lines[start-1:end], "\n"), nil
}
