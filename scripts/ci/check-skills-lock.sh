#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-or-later
#
# check-skills-lock.sh — .agents/skills/ and skills-lock.json must list the
# same skills.
#
# The vendored skills are tracked on purpose and skills-lock.json is the index
# of them, but nothing kept the two in step: removing a skill meant editing
# both by hand, and enable-semantic-release already had a directory with no
# lock entry.
#
# What is compared, in both directions:
#
#   directories — every directory under .agents/skills/ must be a key in
#                 skills-lock.json (the .skills object).
#   lock keys   — every key in skills-lock.json must have a directory.
#
# UNLOCKED lists skills that are deliberately absent from the lock because
# they are written in this repo rather than installed from a source.
#
# Exit 0 when the two agree. Exit 1 listing the differences.
#
# POSIX sh — no bashisms — so it runs identically in local shells, the
# pre-commit hook, and the minimal CI image.

set -eu

ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$ROOT"

# Repo-authored skills with no lock entry (space-separated).
UNLOCKED="enable-semantic-release"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

find .agents/skills -mindepth 1 -maxdepth 1 -type d -exec basename {} \; | sort >"$tmp/dirs"
jq -r '.skills | keys[]' skills-lock.json | sort >"$tmp/lock"

for name in $UNLOCKED; do
	grep -vx "$name" "$tmp/dirs" >"$tmp/dirs.next" || true
	mv "$tmp/dirs.next" "$tmp/dirs"
done

missing_from_lock="$(comm -23 "$tmp/dirs" "$tmp/lock")"
missing_dir="$(comm -13 "$tmp/dirs" "$tmp/lock")"

status=0
if [ -n "$missing_from_lock" ]; then
	echo "skills with a directory but no skills-lock.json entry:" >&2
	echo "$missing_from_lock" | sed 's/^/  /' >&2
	status=1
fi
if [ -n "$missing_dir" ]; then
	echo "skills-lock.json entries with no .agents/skills/ directory:" >&2
	echo "$missing_dir" | sed 's/^/  /' >&2
	status=1
fi

exit "$status"
