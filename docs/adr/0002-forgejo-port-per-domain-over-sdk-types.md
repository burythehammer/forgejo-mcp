<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
# 0002. Forgejo port per domain, over SDK types

Status: accepted

## Context

Tool functions reach Forgejo through the package-level `forgejo.Client(ctx)` singleton, mixing the Forgejo SDK and raw HTTP calls. Tool functions cannot be tested without a live or stubbed HTTP server, and the SDK leaks into every domain. The restructure (spec #7) puts a Forgejo port between the application and Forgejo, in the hexagonal style: the application depends on an interface it owns, and the real Forgejo adapter and an in-memory fake both implement it.

## Decision

- **One port per domain** (issues, pull requests, and so on), with methods named for what the tools need. A top-level Forgejo factory composes them and is built once in the composition root.
- **SDK types appear at the port.** Domain types of our own are deferred to #8, which will decide them.
- **The SDK client and raw HTTP stay inside the adapter.** No SDK client is built and no HTTP to Forgejo is made outside `pkg/forge/sdk`. SDK types appear as data at the port (and in `operation/*`) until #8 replaces them with domain types.

## Alternatives considered

- **One wide interface.** A single `Forgejo` interface with every method. Rejected: every fake and every consumer would depend on all of it, so unrelated domains would change together and merge conflicts would concentrate in one file.
- **Per-use-case interfaces.** One small interface per tool or use case. Rejected: it multiplies interfaces and fakes (over a hundred tools), duplicates method sets across them, and gains little over domain-sized ports.
- **Domain types at the port now.** Translate to repo-owned types immediately. Rejected for now: it doubles the size of the restructure and pre-empts the decision #8 exists to make; SDK types at the port keep the move mechanical, and can be replaced later behind the same port.
- **Leave the SDK visible to tool functions.** Keep calling the SDK and raw HTTP directly. Rejected: it is the coupling the port exists to remove, and it makes a stateful fake impossible.

## Consequences

- We gain tests that run against a stateful in-memory fake, and one contract suite that checks the fake and the real adapter agree.
- SDK upgrades are contained to the adapter, apart from the SDK types at the port.
- Until #8 lands, SDK types still leak through the port, so callers are coupled to SDK struct shapes.
- Each domain needs its own port, fake and contract tests, which is more files than one wide interface.
