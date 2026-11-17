<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
# Configuration

Everything beyond the stdio setup in the [README](../README.md): the HTTP and SSE transports, exposing the server to a network, authentication, OAuth resource-server mode, TLS to Forgejo, attachment uploads, every option, and troubleshooting.

## Transports and access

**For streamable HTTP mode** (recommended for remote/Claude.ai):

```json
{
  "mcpServers": {
    "forgejo": {
      "url": "http://localhost:8080/mcp"
    }
  }
}
```

When using streamable HTTP mode, start the server first:

```bash
forgejo-mcp --transport http --url https://your-forgejo-instance.org --token <your-token>
```

**Multi-tenant HTTP mode** (optional):

You can run a single centralized `forgejo-mcp` instance and let each client provide its own token via the standard HTTP `Authorization` header. This enables serving multiple users or agents from one server.

1. Start the server (optionally without any global token):
   ```bash
   forgejo-mcp --transport http --url https://your-forgejo-instance.org
   ```
2. Clients include their specific token in each request:
   - `Authorization: token <token>` (Forgejo style)
   - `Authorization: Bearer <token>` (OAuth2/MCP style)
   - Note: The scheme (`token` or `Bearer`) is case-insensitive.

See [demos/multi-tenant-http.md](../demos/multi-tenant-http.md) for a copy-pasteable walkthrough.

### Exposing the server to a network

By default the `sse` and `http` transports listen on loopback only, so nothing outside
this machine can reach them. This follows the Model Context Protocol's guidance for
locally-run servers, and it is a change in behaviour: earlier versions listened on
every network interface. The default, `localhost`, binds both loopback families, so a
client that resolves `localhost` to either `127.0.0.1` or `::1` connects. If the port is
already taken on either family the server refuses to start, rather than serve on the
other family while some clients reach whatever holds the port. A family the machine
cannot use at all — IPv6 disabled, say — is skipped, and the startup log says so.

Pass an address rather than a name — `--host 127.0.0.1` or `--host ::1` — to bind that
family alone. Use it when the other family is unusable on this machine in a way the
server cannot recognise as "absent", so that it would otherwise refuse to start.

**If you run the server in a container, or serve remote clients, you must now say so
explicitly.** The default will not accept connections from outside the machine — or,
in a container, from outside the container:

- set `--host` to an address the network can reach (`0.0.0.0` for all interfaces,
  which is the usual choice inside a container);
- and set `--allowed-hosts` to the host names your clients use. This is required, not
  optional: the server refuses to start on a network-reachable address without it,
  rather than starting and rejecting every request.

Requests are refused with `403 Forbidden` when their `Host` header is not one you
declared. Loopback names are always accepted on a loopback listener, so declaring a
proxy's hostname does not stop you connecting directly from the same machine.

**Browser clients.** A request carrying an `Origin` header is refused unless that
origin is listed in `--allowed-origins`, which is empty by default. Origins are
compared in full — scheme, host and port — because an `Origin`'s port belongs to the
page making the request, not to this server. Requests with no `Origin` header at all
are unaffected, which is the normal case: an MCP client is not a browser.

### Request body size on `sse` and `http`

Every request body on these transports is capped at `--max-request-body-bytes`
(default 96 MiB, `100663296`). A request over the cap is refused with `413 Request
Entity Too Large` before authentication runs, and the body names the limit. The
cap applies to both the SSE and the streamable HTTP endpoints, in passthrough and
resource-server modes alike. A normal request, including a base64 attachment upload
up to the 90 MiB upload cap, is unaffected. Raise the cap only if a deployment needs
a larger request, since the server reads each body into memory. `stdio` has no body
cap.

### Authentication on `sse` and `http`

On these transports every request must carry its own `Authorization` header. A request
without one is refused with `401 Unauthorized`, rather than being served using the
server's own configured token. This is what the multi-tenant setup above expects
anyway, and it is why that setup is safe to expose.

On `stdio` the configured token still stands in for an absent header exactly as
before. Nothing about `stdio` changes.

If you run a single-user deployment over `sse` or `http` and want the old behaviour,
`--allow-operator-token-fallback` restores it. Understand what it means before you use
it: any client that can reach the port acts as the identity behind your token, without
presenting a credential. The startup log says so, loudly, whenever it is on.

**For SSE mode** (legacy HTTP-based):

```json
{
  "mcpServers": {
    "forgejo": {
      "url": "http://localhost:8080/sse"
    }
  }
}
```

When using SSE mode, start the server first:

```bash
forgejo-mcp --transport sse --url https://your-forgejo-instance.org --token <your-token>
```

### Remote operation as an OAuth resource server

With `--auth-mode resource-server`, the `http` transport becomes an OAuth 2.0
resource server as the MCP authorization specification defines it. Clients no
longer send a Forgejo token. They log in at an OpenID Connect provider you choose
and send its JWT access token. The server validates that token, signs a JWT that
lives five minutes for the caller, and presents it to Forgejo through a Forgejo 16
Authorized Integration. No forge token is configured anywhere, and the caller's
access token never reaches Forgejo.

The mode needs:

- Forgejo 16.0 or newer;
- an OpenID Connect provider that issues JWT access tokens and can add a per-user
  claim (default `forgejo_aud`) holding the audience of that user's Authorized
  Integration;
- a public HTTPS origin for the server, because Forgejo fetches the server's issuer
  documents from it;
- a signing key file: EC P-256 or P-384, Ed25519, or RSA of at least 2048 bits.

```bash
forgejo-mcp --transport http --url https://forgejo.example.org \
  --host 0.0.0.0 --allowed-hosts mcp.example.org \
  --auth-mode resource-server \
  --authorization-server https://id.example.org \
  --resource https://mcp.example.org/mcp \
  --forgejo-jwt-issuer https://mcp.example.org/issuer \
  --forgejo-jwt-signing-key-file /run/credentials/forgejo-mcp.service/signing-key
```

The mode is opt-in: without `--auth-mode`, everything above behaves as before. In
this mode the server answers only `/mcp`, the protected resource metadata, and the
discovery document and key set under the issuer path. It refuses to start on a
configuration it cannot serve safely, and the refusal names the setting to fix.
The signing key acts for every user whose integration trusts the issuer, so guard
it like the forge token it replaces.

- [Operator guide](oauth-resource-server/operator.md): identity provider
  requirements, deployment order, proxy rules, key rotation, and a worked Zitadel
  example.
- [User guide](oauth-resource-server/user.md): creating the Authorized
  Integration, the mandatory `sub` claim rule, and configuring an MCP client.

## Options

You can configure the server using command-line arguments or environment variables:

| CLI Argument | Environment Variable | Description |
|--------------|---------------------|-------------|
| `--url` | `FORGEJO_URL` | Your Forgejo instance URL |
| `--token` | `FORGEJO_ACCESS_TOKEN` | Your personal access token |
| `--debug` | `FORGEJO_DEBUG` | Enable debug mode |
| `--transport` | - | Transport mode: `stdio`, `sse`, or `http` |
| `--sse-port` | - | Port for SSE mode (default: 8080) |
| `--http-port` | - | Port for streamable HTTP mode (default: 8080) |
| `--shutdown-grace` | `FORGEJO_MCP_SHUTDOWN_GRACE` | On SIGINT or SIGTERM, how long in-flight `sse`/`http` requests may run before open connections are closed (default: `8s`; any positive Go duration such as `30s` or `2m`, validated at startup). At the deadline, open event streams are closed, so clients see the stream end and should reconnect; the server then exits with status 0 and logs a warning. Set it below your orchestrator's termination timeout |
| `--host` | `FORGEJO_MCP_HOST` | Address the `sse` and `http` transports bind to (default: `localhost`, reachable from this machine only, binding both `127.0.0.1` and `::1`; pass one of those addresses to bind that family alone) |
| `--allowed-hosts` | `FORGEJO_MCP_ALLOWED_HOSTS` | Comma-separated `Host` names this server answers to; required when `--host` is not loopback |
| `--toolsets` | `FORGEJO_MCP_TOOLSETS` | Comma-separated toolsets to load; unset loads every tool. See [Toolsets](reference/toolsets.md#toolsets-choosing-which-tools-load) |
| `--tools` | `FORGEJO_MCP_TOOLS` | Comma-separated individual tool names to load, added to any `--toolsets` |
| `--preset` | `FORGEJO_MCP_PRESET` | Named cross-domain tool selection, added to any `--toolsets` and `--tools`. The only preset is `tracker-core`; see [Presets](reference/toolsets.md#presets) |
| `--lean` | `FORGEJO_MCP_LEAN` | Serve terse tool text with the same tool and argument names; see [Lean mode](reference/tools.md#lean-mode-terse-tool-text) |
| `--lax-args` | `FORGEJO_MCP_LAX_ARGS` | Serve a call that names an argument the tool does not declare, dropping that argument, as every tool did before. By default `search_issues` refuses such a call with an invalid-argument error that names each unknown argument and lists the valid ones, before any request reaches Forgejo (a `query` for `q` used to return an unfiltered list). Other tools already ignore unknown arguments; this flag affects only `search_issues` |
| `--allowed-origins` | `FORGEJO_MCP_ALLOWED_ORIGINS` | Comma-separated web origins allowed to send an `Origin` header, as full origins (`https://console.example.org`). Empty by default |
| `--allow-operator-token-fallback` | `FORGEJO_MCP_ALLOW_OPERATOR_TOKEN_FALLBACK` | On `sse`/`http`, serve requests with no `Authorization` header using this server's own token. Off by default |
| `--max-request-body-bytes` | `FORGEJO_MCP_MAX_REQUEST_BODY_BYTES` | On `sse`/`http`, the largest request body read, in bytes; a larger one gets `413`. Must be positive. Default `100663296` (96 MiB), which clears the largest attachment upload (90 MiB of base64) plus its envelope |
| `--auth-mode` | `FORGEJO_MCP_AUTH_MODE` | `passthrough` (default) or `resource-server`; see [Remote operation as an OAuth resource server](#remote-operation-as-an-oauth-resource-server) |
| `--authorization-server` | `FORGEJO_MCP_AUTHORIZATION_SERVER` | `resource-server` mode: issuer URL of the OpenID Connect provider, compared byte for byte |
| `--authorization-server-tls-ca` | `FORGEJO_MCP_AUTHORIZATION_SERVER_TLS_CA` | `resource-server` mode: PEM file of CA certificates trusted for the identity provider's discovery and key lookups, added to the system roots. Separate from `--tls-ca`; the server refuses to start if the file cannot be read or holds no PEM certificates |
| `--resource` | `FORGEJO_MCP_RESOURCE` | `resource-server` mode: canonical URI of the MCP endpoint, for example `https://mcp.example.org/mcp`; its path must be `/mcp` |
| `--resource-audience` | `FORGEJO_MCP_RESOURCE_AUDIENCE` | `resource-server` mode: value an access token's `aud` must contain (default: the value of `--resource`) |
| `--scopes-supported` | `FORGEJO_MCP_SCOPES_SUPPORTED` | `resource-server` mode: space-separated scopes published in the metadata and the `401` challenge (default: none published) |
| `--forgejo-audience-claim` | `FORGEJO_MCP_FORGEJO_AUDIENCE_CLAIM` | `resource-server` mode: access-token claim holding the audience of the caller's Forgejo Authorized Integration (default: `forgejo_aud`) |
| `--forgejo-jwt-issuer` | `FORGEJO_MCP_FORGEJO_JWT_ISSUER` | `resource-server` mode: issuer URL under which the server signs JWTs for Forgejo, for example `https://mcp.example.org/issuer`; https, without a trailing slash |
| `--forgejo-jwt-signing-key-file` | `FORGEJO_MCP_FORGEJO_JWT_SIGNING_KEY_FILE` | `resource-server` mode: PEM private key that signs the JWTs for Forgejo (EC P-256 or P-384, Ed25519, or RSA of at least 2048 bits) |
| `--forgejo-jwt-published-key-files` | `FORGEJO_MCP_FORGEJO_JWT_PUBLISHED_KEY_FILES` | `resource-server` mode: comma-separated PEM keys published next to the signing key, for key rotation |
| `--cli` | - | Enter CLI mode for direct tool invocation |
| `--user-agent` | `FORGEJO_USER_AGENT` | HTTP User-Agent header (default: `forgejo-mcp/<version>`) |
| `--request-timeout` | `FORGEJO_REQUEST_TIMEOUT` | Longest wait for one Forgejo request, as a Go duration such as `30s` (default: `60s`). A stalled Forgejo fails the call with a "did not respond in time" error. Uploads have their own bound of 10 minutes |
| `--tls-ca` | `FORGEJO_TLS_CA` | PEM file of CA certificates to trust for the Forgejo instance, added to the system roots (see [Private or self-signed Forgejo certificates](#private-or-self-signed-forgejo-certificates)) |
| `--tls-skip-verify` | `FORGEJO_TLS_SKIP_VERIFY` | Turn off TLS certificate verification for the Forgejo instance. Off by default; logs a warning at startup (see [Private or self-signed Forgejo certificates](#private-or-self-signed-forgejo-certificates)) |
| - | `FORGEJO_MCP_ALLOW_FILE_PATH_UPLOAD` | Allow `file_path` attachment uploads to read the host filesystem (`1`/`true`/`yes`/`on`; off by default) |
| - | `FORGEJO_MCP_UPLOAD_ROOT` | Confine `file_path` uploads to this directory (default: anywhere the process can read) |

Command-line arguments take priority over environment variables.

The `resource-server` settings are refused in `passthrough` mode, so a configuration that sets them but forgets `--auth-mode resource-server` does not start.

### Private or self-signed Forgejo certificates

If your Forgejo instance uses a private CA or a self-signed certificate, the server fails certificate verification on every request by default. Prefer the first option.

- **Trust your CA (recommended).** Point `FORGEJO_TLS_CA` (or `--tls-ca`) at a PEM file holding the CA certificate. It is added to the system roots, not substituted for them, so public certificates still verify. The server refuses to start if the file cannot be read or holds no PEM certificates.
- **Skip verification (testing only).** Set `FORGEJO_TLS_SKIP_VERIFY=true` (or `--tls-skip-verify`). The server then accepts any certificate from the Forgejo instance. It logs a warning at startup.

The risk of skip-verify: the server no longer checks who it is talking to. Anyone who can intercept traffic between the server and Forgejo can read and alter it, including the access token the server sends. Use it only for a throwaway instance on a network you trust. Do not use it against a production instance.

Both settings apply to every request the server makes to Forgejo, through the SDK client and the raw-HTTP paths alike (JSON, raw, multipart and download). They do not affect the identity provider's lookups in `resource-server` mode; those have their own setting, `--authorization-server-tls-ca` (see [Remote operation as an OAuth resource server](#remote-operation-as-an-oauth-resource-server)). Skip-verify has no provider counterpart.

### Uploading attachments from the host filesystem

`create_issue_attachment`, `create_comment_attachment`, and
`create_release_attachment` accept either base64 `content` or a `file_path` on
the machine running `forgejo-mcp`. The path form avoids base64-expanding a large
release artifact through the MCP transport.

It is **off by default**, because it hands whatever drives the MCP client the
ability to read any file the server process can read — a prompt-injected agent
could upload `~/.ssh/id_ed25519` as a public release asset. Turn it on
deliberately, and prefer confining it:

```bash
export FORGEJO_MCP_ALLOW_FILE_PATH_UPLOAD=1
export FORGEJO_MCP_UPLOAD_ROOT=/home/you/build/dist   # optional but recommended
```

With `FORGEJO_MCP_UPLOAD_ROOT` set, a path that resolves outside that directory
— by being absolute, by `..`, or through a symlink — is rejected before anything
is read. Base64 `content` uploads are unaffected by either variable.

## Troubleshooting

**Enable debug mode** to see detailed logs:

```bash
forgejo-mcp --transport sse --url <url> --token <token> --debug
```

Or set the environment variable:

```bash
export FORGEJO_DEBUG=true
```

**Custom User-Agent**: If your Forgejo instance or proxy blocks the default `go-http-client` user agent, set a custom one:

```bash
# Via environment variable
export FORGEJO_USER_AGENT="forgejo-mcp/1.0.0"

# Or via CLI flag
forgejo-mcp --user-agent "forgejo-mcp/1.0.0" --transport sse --url <url> --token <token>
```
