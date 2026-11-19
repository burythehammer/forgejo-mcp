#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-or-later
#
# check-skills-lock.sh — .agents/skills/ and skills-lock.json must list the
# same skills.
#
# The vendored skills are tracked on purpose and skills-lock.json is the index
# of them, but nothing kept the two in step: removing a skill meant editing
# both by hand.
#
# What is compared, in both directions:
#
#   directories — every directory under .agents/skills/ must be a key in
#                 skills-lock.json (the .skills object).
#   lock keys   — every key in skills-lock.json must have a directory.
#
# Exit 0 when the two agree. Exit 1 listing the differences.
#
# POSIX sh — no bashisms — so it runs identically in local shells, the
# pre-commit hook, and the minimal CI image.

set -eu

ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$ROOT"

command -v jq >/dev/null || { echo "check-skills-lock: jq is required but not installed" >&2; exit 1; }

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

find .agents/skills -mindepth 1 -maxdepth 1 -type d -exec basename {} \; | sort >"$tmp/dirs"
jq -r '.skills | keys[]' skills-lock.json | sort >"$tmp/lock"

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
