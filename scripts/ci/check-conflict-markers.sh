#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-or-later
#
# check-conflict-markers.sh — reject committed merge-conflict markers.
#
# During #7, conflict-marker cleanup left invalid Go behind and nothing caught
# it until much later. Only the opening and closing markers are matched: a
# bare "=======" line is legitimate in Markdown and RST.
#
# Exit 0 when clean, 1 listing each marker line.

set -eu

cd "$(git rev-parse --show-toplevel 2>/dev/null || dirname "$0"/../..)"

# Built from pieces so this file does not match itself.
open='^<<<<'"<<< "
close='^>>>>'">>> "

hits=$(git ls-files -z | xargs -0 grep -InE -e "$open" -e "$close" 2>/dev/null |
	grep -v '^scripts/ci/check-conflict-markers\.sh:' || true)

if [ -n "$hits" ]; then
	echo "error: merge-conflict marker(s) found:" >&2
	echo "" >&2
	echo "$hits" >&2
	echo "" >&2
	echo "Resolve the conflict, then run 'go build ./... && go vet ./...'." >&2
	exit 1
fi

exit 0
