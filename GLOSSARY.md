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
An implementation of the Forgejo port. The real one talks to a Forgejo instance, using whichever transport is needed; the fake one keeps state in memory.
_Avoid_: backend, driver
