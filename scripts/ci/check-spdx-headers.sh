#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-or-later
#
# check-spdx-headers.sh — every tracked Go and shell source file must carry an
# SPDX-License-Identifier line near the top (AGENTS.md "File Header").
#
# Reviewers sampled rather than read every file in large diffs, so the rule
# went unchecked. Vendored skill trees (.agents/, .claude/) are not ours and
# are skipped.
#
# Exit 0 when clean, 1 listing files with no header.

set -eu

cd "$(git rev-parse --show-toplevel 2>/dev/null || dirname "$0"/../..)"

missing=""
for f in $(git ls-files '*.go' '*.sh' | grep -v -e '^\.agents/' -e '^\.claude/'); do
	[ -f "$f" ] || continue
	if ! head -3 "$f" | grep -q 'SPDX-License-Identifier:'; then
		missing="$missing$f
"
	fi
done

if [ -n "$missing" ]; then
	echo "error: source file(s) missing the SPDX-License-Identifier header:" >&2
	echo "" >&2
	printf '%s' "$missing" >&2
	echo "" >&2
	echo "Add '// SPDX-License-Identifier: GPL-3.0-or-later' (Go) or" >&2
	echo "'# SPDX-License-Identifier: GPL-3.0-or-later' (shell) as the first" >&2
	echo "lines of the file, after any shebang." >&2
	exit 1
fi

exit 0
