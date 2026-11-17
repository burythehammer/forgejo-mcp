#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-or-later
#
# check-doc-tool-names.sh — agent docs may only cite tools that are registered.
#
# AGENTS.md and docs/agents/ tell agents which MCP tools to call. Nothing
# checked those names, so a rename or removal left the docs naming a tool that
# no longer exists; the tool list in docs/agents/issue-tracker.md was first
# written from a grep and from memory.
#
# The registered set is the same one check-readme-tool-coverage.sh uses: every
# `<Name>ToolName = "<tool_name>"` constant under internal/operation/.
#
# Two rules, because the docs cite tools in two ways:
#
#   prefixed — every `mcp__forgejo__<name>` in AGENTS.md, README.md and docs/,
#              including the reference tables under docs/reference/
#              must name a registered tool. The `mcp__forgejo__*` wildcard is
#              a server reference, not a tool, and is skipped.
#   bare     — in docs/agents/*.md only, a backticked snake_case token whose
#              first word is the first word of some registered tool
#              (create_, get_, list_, ...) must itself be registered. Tokens
#              that merely look like tools but are parameters go in
#              PARAM_NAMES below.
#
# The bare rule is a heuristic and is limited to docs/agents/ so that prose
# elsewhere (design notes, plans naming tools that do not exist yet) is not
# held to it.
#
# Exit 0 when every cited name is registered. Exit 1 listing what is not.
#
# POSIX sh — no bashisms — so it runs identically in local shells, the
# pre-commit hook, and the minimal CI image.

set -eu

# Byte order, so sort and comm agree regardless of the caller's locale.
LC_ALL=C
export LC_ALL

ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$ROOT"

# Parameter or field names that start like a tool name (space-separated).
PARAM_NAMES="set_labels delete_branch_after_merge"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

grep -rhE '^[[:space:]]*[A-Za-z]+ToolName[[:space:]]*=[[:space:]]*"[a-z_]+"' \
  internal/operation/ --include='*.go' 2>/dev/null \
  | sed -n 's/.*"\([a-z_]*\)".*/\1/p' \
  | sort -u >"$tmp/tools"

sed 's/_.*//' "$tmp/tools" | sort -u >"$tmp/verbs"

# --- prefixed --------------------------------------------------------------
grep -rhoE 'mcp__forgejo__[a-z][a-z_]*' AGENTS.md README.md docs 2>/dev/null \
  | sed 's/^mcp__forgejo__//' \
  | sort -u >"$tmp/prefixed"
comm -23 "$tmp/prefixed" "$tmp/tools" >"$tmp/prefixed_bad"

# --- bare ------------------------------------------------------------------
: >"$tmp/bare"
for f in docs/agents/*.md; do
  [ -f "$f" ] || continue
  grep -oE '`[a-z]+(_[a-z]+)+`' "$f" | tr -d '`' >>"$tmp/bare"
done
sort -u "$tmp/bare" >"$tmp/bare.sorted"

: >"$tmp/bare_bad"
while IFS= read -r name; do
  verb="${name%%_*}"
  grep -qx "$verb" "$tmp/verbs" || continue
  grep -qx "$name" "$tmp/tools" && continue
  case " $PARAM_NAMES " in *" $name "*) continue ;; esac
  echo "$name" >>"$tmp/bare_bad"
done <"$tmp/bare.sorted"

# --- report ----------------------------------------------------------------
tools_total="$(wc -l <"$tmp/tools" | tr -d ' ')"
bad_prefixed="$(wc -l <"$tmp/prefixed_bad" | tr -d ' ')"
bad_bare="$(wc -l <"$tmp/bare_bad" | tr -d ' ')"

if [ "$bad_prefixed" -gt 0 ]; then
  echo "mcp__forgejo__ names in the docs that are not registered tools:" >&2
  sed 's/^/  - /' "$tmp/prefixed_bad" >&2
fi
if [ "$bad_bare" -gt 0 ]; then
  echo "Tool-like names in docs/agents/ that are not registered tools:" >&2
  sed 's/^/  - /' "$tmp/bare_bad" >&2
fi

errors=$((bad_prefixed + bad_bare))

echo "check-doc-tool-names: ${tools_total} registered tool(s), ${errors} unknown name(s) cited."

if [ "$errors" -gt 0 ]; then
  echo "" >&2
  echo "Fix the name, or if it is a parameter and not a tool add it to PARAM_NAMES in this script." >&2
  exit 1
fi
exit 0
