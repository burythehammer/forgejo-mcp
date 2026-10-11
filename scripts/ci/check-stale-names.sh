#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-or-later
#
# check-stale-names.sh — live docs must not cite names from the removed
# pkg/forgejo package.
#
# After the Forgejo port (#60) the raw-HTTP helpers and client globals moved
# into pkg/forge, but references survived in live specs and docs. Historical
# records (docs/plans, openspec/changes) legitimately keep the old names and
# are not checked.
#
# Exit 0 when clean, 1 listing each stale reference.

set -eu

cd "$(git rev-parse --show-toplevel 2>/dev/null || dirname "$0"/../..)"

pattern='forgejo\.Client\(|DoJSON|DoAPIRaw|SetClientForTesting'

hits=$(grep -rnE "$pattern" openspec/specs AGENTS.md DEVELOPER.md 2>/dev/null || true)

if [ -n "$hits" ]; then
	echo "error: stale reference(s) to the removed pkg/forgejo package:" >&2
	echo "" >&2
	echo "$hits" >&2
	echo "" >&2
	echo "Update the doc to the pkg/forge equivalent. docs/plans and" >&2
	echo "openspec/changes are historical and are not checked." >&2
	exit 1
fi

exit 0
