<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
# 0004. Dependency licenses are checked against an allowlist

Status: accepted

## Context

The inherited Tekton `license-check` forbade GPL and LGPL dependencies "to protect the Apache-2.0 binary". This project is `GPL-3.0-or-later`, so that rule protects a license the project no longer ships under, and its `--ignore` still named the old `b4mad` module path.

## Decision

CI fails a dependency whose license is not on an allowlist: permissive licenses (MIT, BSD, Apache-2.0, ISC), MPL-2.0, LGPL and GPL-3.0-compatible licenses. AGPL, GPL-2.0-only, SSPL, proprietary and `unknown` fail. `go-licenses` is pinned, and the check runs in the Forgejo Actions gate and in `task check`.

## Consequences

An allowlist fails closed: a new dependency with an unrecognised license blocks the build until someone classifies it, which is the intent.
