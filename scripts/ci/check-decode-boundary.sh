#!/bin/sh
# SPDX-License-Identifier: GPL-3.0-or-later
#
# check-decode-boundary.sh — a tool's arguments are decoded only by the request
# pipeline (#354, closing the #333 migration).
#
# internal/operation/pipeline decodes every tool's arguments, runs Validate, and
# only then calls the Binder that looks up a credential. A domain registers its
# tools with pipeline.Handle. A domain that calls decode.Lax or decode.Args itself
# skips Validate and the refusal-before-credential ordering, so this script fails
# on any such call in a non-test file outside internal/operation/pipeline and
# internal/operation/decode (the decode package's own code and tests are exempt).
#
# Usage: check-decode-boundary.sh
# Exit 0 when clean, 1 on a violation.

set -eu

cd "$(dirname "$0")/../.."

hits=$(find internal -name '*.go' ! -name '*_test.go' \
	! -path 'internal/operation/pipeline/*' ! -path 'internal/operation/decode/*' \
	-exec grep -nE '\bdecode\.(Lax|Args)\(' {} + 2>/dev/null || true)

if [ -n "$hits" ]; then
	echo "error: decode.Lax or decode.Args called outside internal/operation/pipeline. Declare the argument struct and register the tool with pipeline.Handle (Validate, Binder, Lax/Strict/WithRaw):" >&2
	echo "$hits" | sed 's/^/  /' >&2
	exit 1
fi
