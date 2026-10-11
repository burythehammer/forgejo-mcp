#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-or-later
#
# check-app-boundary.sh — keep the use-case layer MCP-free and SDK-free.
#
# Packages under app/ are use cases (docs/adr/0003): they take plain inputs,
# reach Forgejo only through the forgejo port in pkg/forge, and return
# repo-owned result types. An MCP adapter in operation/ translates to and from
# MCP. So no non-test or test Go file under app/ may import mcp-go
# (github.com/mark3labs/mcp-go) or the Forgejo SDK (codeberg.org/mvdkleijn/
# forgejo-sdk, or any forgejo-sdk path).
#
# Exit 0 when clean, 1 when a forbidden import is found.

set -eu

cd "$(dirname "$0")/../.."

[ -d app ] || exit 0

hits=$(grep -rnE '"(github\.com/mark3labs/mcp-go|[^"]*forgejo-sdk)[^"]*"' --include='*.go' app || true)

if [ -n "$hits" ]; then
	echo "error: app/** imports mcp-go or the Forgejo SDK." >&2
	echo "" >&2
	echo "$hits" >&2
	echo "" >&2
	echo "Use cases must stay MCP-free: take plain inputs, use the forgejo port" >&2
	echo "(pkg/forge) and return repo-owned types. See docs/adr/0003." >&2
	exit 1
fi

exit 0
