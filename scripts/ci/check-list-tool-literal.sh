#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-or-later
#
# check-list-tool-literal.sh — truncation pointers must name only registered tools.
#
# A resource that is cut tells the caller which tool fetches the rest
# (docs/agents/resources.md). Assigning that tool name as a bare string literal
# (payload.ListTool = "list_repo_hooks") skips resource.ToolRef, which drops the
# name when no such tool is registered. Write
# payload.ListTool = resource.ToolRef("list_repo_hooks") instead.
#
# Exit 0 when clean, 1 when a bare literal is assigned to a *ListTool field.

set -eu

cd "$(dirname "$0")/../.."

hits=$(grep -rnE 'ListTool[[:space:]]*=[[:space:]]*"' --include='*.go' operation \
	| grep -v '_test\.go:' | grep -v '^operation/resource/' || true)

if [ -n "$hits" ]; then
	echo "error: a *ListTool field is assigned a bare string literal." >&2
	echo "" >&2
	echo "$hits" >&2
	echo "" >&2
	echo 'Wrap it: resource.ToolRef("tool_name"). See docs/agents/resources.md.' >&2
	exit 1
fi
