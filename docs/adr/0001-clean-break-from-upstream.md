<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
# 0001. Clean break from upstream

Status: accepted

## Context

This repository is a fork of b4mad/forgejo-mcp (`upstream`). The planned architecture restructure moves configuration into a typed Config built by a composition root, puts a Forgejo port behind the handlers, splits use cases from the MCP adapter, and extracts transport and auth. These changes touch almost every file under `operation/`, `cmd/` and `pkg/`.

## Decision

This fork restructures freely and does not preserve mergeability with b4mad upstream. Upstream changes are no longer merged wholesale; anything wanted from upstream is ported by hand.

## Alternatives considered

- **Stay mergeable.** Keep the layout close enough that upstream merges stay clean. Rejected: the restructure rewrites the call sites that nearly every upstream change touches, so this would either block the restructure or reduce it to cosmetic moves.
- **Restructure only where upstream is quiet.** Leave actively changing areas alone. Rejected: the globals and handlers to be replaced are spread across all domains, so no quiet subset forms a coherent seam, and the result would be two architectures in one tree.

## Consequences

- We gain freedom to choose the layout, with no constraint from upstream file paths or package names.
- We lose cheap merges: each upstream fix or feature must be re-applied manually and may need translating to the new structure.
- Divergence grows over time, so the cost of ever reversing this decision rises with each restructuring ticket.
