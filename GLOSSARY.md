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
An implementation of the Forgejo port. The real one (`pkg/forge/sdk`) talks to a Forgejo instance, using whichever transport is needed; the fake one (`pkg/forge/fake`) keeps state in memory. Not to be confused with a port bridge, which adapts a use case's port rather than implementing Forgejo's.
_Avoid_: backend, driver, real adapter (say "Forgejo adapter")

**Port bridge**:
The package-level translator (`pkg/forge/<domain>adapter/`) that makes a Forgejo port satisfy a use case's own port: it converts the use case's inputs to SDK options and the SDK results back to repo-owned types. It is the only place SDK and repo-owned types meet. It is not a Forgejo adapter: it implements no Forgejo operation, it calls one.
_Avoid_: adapter (unqualified), Bridge in prose

**Use case**:
The MCP-free unit of application logic behind one tool: takes plain inputs, reaches Forgejo only through the Forgejo port, and returns a repo-owned result type. It lives under `app/<domain>/`, where `service.go` holds the domain's use cases as methods on a `Service` type. "Service" is that Go type name only; the concept is the use case.
_Avoid_: handler, service, operation

**MCP adapter**:
The thin layer that exposes a use case as an MCP tool: it owns the tool definition and input schema, decodes arguments into the tool's argument struct, calls the use case, and renders its result as MCP output. It lives under `operation/<domain>/`, in `handlers.go`, and its struct is named `handlers`. Those file and type names are the only place "handler" appears in the code, and they mean this layer.
_Avoid_: handler (as the concept), tool function, adapter (unqualified)

**Compact result**:
The default summary shape of a tool's entity. The write tools return it, and so do the entities in a list result and the single reads of an issue, a pull request, a comment and a review. Passing `verbose: true` returns the full entity instead.
_Avoid_: short result, slim response
