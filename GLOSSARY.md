<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
# Glossary

**Issue ref**:
The branch an issue is linked to, held in Forgejo's own `ref` field on the issue. It is not a pull request link: a pull request that closes the issue is a separate relationship.
_Avoid_: branch link, issue branch

**Config**:
The single validated value holding every operator-supplied setting of a running server, built once at startup and passed down explicitly.
_Avoid_: settings, flags, options, global flags

**Composition root**:
The one place that builds Config and wires the application's parts; nothing below it reads settings from package-level state.
_Avoid_: bootstrap, setup, init

**Forgejo port**:
The interface through which the application talks to a Forgejo instance, expressed in the operations the tools need and split by domain (issues, pull requests, and so on). The real Forgejo and the in-memory fake both satisfy it.
_Avoid_: client, Forgejo client, API wrapper

**Forgejo adapter**:
An implementation of the Forgejo port. The real one (`internal/forge/sdk`) talks to a Forgejo instance, using whichever transport is needed; the fake one (`internal/forge/fake`) keeps state in memory. Not to be confused with a port bridge, which adapts a use case's port rather than implementing Forgejo's.
_Avoid_: backend, driver, real adapter (say "Forgejo adapter")

**Port bridge**:
The package-level translator (`internal/forge/<domain>adapter/`) that makes a Forgejo port satisfy a use case's own port: it converts the use case's inputs to SDK options and the SDK results back to repo-owned types. It is the only place SDK and repo-owned types meet. It is not a Forgejo adapter: it implements no Forgejo operation, it calls one.
_Avoid_: adapter (unqualified), Bridge in prose

**Tool**:
An MCP tool the server offers, standing for exactly one Forgejo operation. A sequence of operations that serves a workflow is not a tool; it belongs to a skill.
_Avoid_: workflow tool, compound tool, macro

**Toolset**:
A named group of tools for one domain, such as issues or labels. Each tool is in exactly one toolset. Resource templates follow their toolset.
_Avoid_: profile, bundle, tier

**Preset**:
A named selection of individual tools that may cross domains, such as `tracker-core`. A preset holds no toolsets and registers no resource templates; it combines with toolsets and individual tools as a union.
_Avoid_: profile, bundle, tier

**Forgejo version**:
The version string the configured Forgejo instance reports at `/api/v1/version`, returned unparsed by `get_forgejo_version`.
_Avoid_: server version, Forgejo server version

**MCP server version**:
The version of this MCP server build (the configured value, `dev` when none), returned by `get_forgejo_mcp_server_version`. It is not the Forgejo version.
_Avoid_: server version, build version

**Use case**:
The MCP-free unit of application logic behind one tool: takes plain inputs, reaches Forgejo only through the Forgejo port, and returns a repo-owned result type. It lives under `internal/app/<domain>/`, where `service.go` holds the domain's use cases as methods on a `Service` type. "Service" is that Go type name only; the concept is the use case.
_Avoid_: handler, service, operation

**MCP adapter**:
The thin layer that exposes a use case as an MCP tool: it owns the tool definition and input schema, decodes arguments into the tool's argument struct, calls the use case, and renders its result as MCP output. It lives under `internal/operation/<domain>/`, in `handlers.go`, and its struct is named `handlers`. Those file and type names are the only place "handler" appears in the code, and they mean this layer.
_Avoid_: handler (as the concept), tool function, adapter (unqualified)

**Request pipeline**:
The one module in the MCP adapter that carries a tool call from its raw arguments to its rendered result: it decodes and validates the arguments, refuses a bad request before any credential lookup or Forgejo call, binds the use case to the request's credential, runs it, and shapes any error. A tool supplies only its argument struct, its use case and how it renders. A rule that applies to every tool, such as rejecting unknown arguments, is made here once. It does not define tools or register them.
_Avoid_: run helper, handler wrapper, middleware

**Compact result**:
The default summary shape of a tool's entity. The write tools return it, and so do the entities in a list result and the single reads of an issue, a pull request, a comment and a review. Passing `verbose: true` returns the full entity instead.
_Avoid_: short result, slim response

**Lean mode**:
The opt-in setting (`--lean`, or `FORGEJO_MCP_LEAN`, the flag winning) that serves terse tool text: each tool description cut to its first sentence, property descriptions removed except where an argument has a format, an allowed-value list, or a rule its name and type do not show, and annotations sent empty. Tool names and argument names are the same as in the full text. It is a rewrite at registration in `internal/lean/`, not a second set of descriptions.
_Avoid_: profile, bundle, tier, terse mode

**Forgejo wiki**:
The wiki a Forgejo repository carries, stored as a git repository beside the code and reached through the wiki tools. It is flat: a slash in a page name is a naming convention, not a parent-child link.
_Avoid_: wiki (unqualified), repo wiki

**Ceiling**:
The most rows one file-output listing may hold, 1,000 issues (`list.FileCeiling`), shared by every tool that takes `outFile`. A listing of exactly the ceiling is written whole; one row more fails with an error naming the ceiling and writes no file. It is never a truncation. Not the instance ceiling (`max_response_items`), which lowers a page's limit.
_Avoid_: cap, limit, max rows

**File output**:
A tool result written to a file instead of returned as a page, for a caller that processes a whole list by script rather than through the model. The `outFile` argument names it: a bare file name under the server's temp directory, with the reply `{path, count, keys}`. Only the stdio transport accepts it, since a network caller cannot read a file on the server host. `with_blockers` (valid only with `outFile`) adds each row's `blocked_by`, read per issue under the same ceiling.
_Avoid_: export, dump, download

**Outline wiki**:
The personal knowledge base kept in Outline, including the per-repo structured wikis the `repo-wiki` skill maintains. It is separate from the Forgejo wiki and holds notes that are not tied to one repository.
_Avoid_: wiki (unqualified), Outline docs
