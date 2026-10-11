<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
# 0003. Tools are split into an MCP adapter and an MCP-free use case, with repo-owned output types

Status: accepted

## Context

Tool handlers in `operation/*` read MCP arguments, call Forgejo, and format MCP results in one function. Business logic cannot be tested without building MCP requests, and SDK and mcp-go types leak through every handler. With the Forgejo port in place (ADR 0002), the remaining coupling is between tool logic and the MCP protocol (spec #8).

## Decision

- **Each tool is an MCP adapter plus a use case.** The adapter (in `operation/<domain>`) owns the tool definition, decodes arguments into a per-tool argument struct with the shared decoder (`operation/decode`), calls the use case, and renders the result as MCP output. The use case (in `app/<domain>`) takes plain inputs, reaches Forgejo only through the Forgejo port, and returns a repo-owned result type.
- **`app/**` is MCP-free and SDK-free.** It must not import mcp-go or the Forgejo SDK; `scripts/ci/check-app-boundary.sh` enforces this in CI.
- **The wire contract is frozen.** Tool names, input schemas and output bytes do not change by moving a tool. Each migrated tool is pinned by golden-output and schema-snapshot tests (`operation/mcptest`) written before the move, and an argument struct is checked against its schema by `mcptest.ArgsMatchSchema`.
- **Repo-owned output types** replace SDK types at the use case boundary. Until a domain migrates, its port still returns SDK types.

## Alternatives considered

- **Keep handlers as they are.** Rejected: logic stays untestable without MCP, and the SDK and mcp-go stay coupled everywhere.
- **Generate the schema from the argument struct.** Rejected for now: it would change schema bytes; the contract is frozen and the struct is verified against the hand-written schema instead.
- **Put use cases beside adapters in `operation/`.** Rejected: the boundary cannot be enforced by a directory-level import check.

## Consequences

- Use cases test against the in-memory fake with no MCP in sight.
- Every tool gains an argument struct, a use case and a result type: more files, in exchange for a mechanical, enforceable boundary.
- The migration proceeds domain by domain; `version` is the first, proving the decoder, test helpers and CI check.
