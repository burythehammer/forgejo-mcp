// SPDX-License-Identifier: GPL-3.0-or-later

package diff

import "strings"

// Files splits a unified diff into one section per `diff --git` header, so a
// caller can page the diff by file. Text before the first header is a section
// of its own. Joining the sections with "\n" gives back rawDiff exactly.
func Files(rawDiff string) []string {
	if rawDiff == "" {
		return nil
	}
	const marker = "diff --git "
	lines := strings.Split(rawDiff, "\n")
	var out []string
	start := 0
	for i := 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], marker) {
			out = append(out, strings.Join(lines[start:i], "\n"))
			start = i
		}
	}
	return append(out, strings.Join(lines[start:], "\n"))
}
