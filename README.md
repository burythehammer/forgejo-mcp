# Forgejo MCP Server

Connect your AI assistant to Forgejo repositories. Manage issues, pull requests, files, and more through natural language.

**Origin:** forgejo-mcp began as a fork of b4mad/forgejo-mcp and has been independent since [ADR 0001](docs/adr/0001-clean-break-from-upstream.md).

## What It Does

Forgejo MCP Server is an integration plugin that connects Forgejo with [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) systems. Once configured, you can work with your Forgejo repositories from any MCP-compatible AI assistant, such as Claude, Cursor, or a VS Code extension.

Examples of what you can ask:

- "List all my repositories"
- "Create an issue titled 'Bug in login page'"
- "Show me open pull requests in my-org/my-repo"
- "Get the contents of README.md from the main branch"
- "Show me the latest Actions workflow runs in my-org/my-app"

## Quick Start

### 1. Install

**Option A: Go install (recommended)**

```bash
go install forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3@latest
```

Ensure `$GOPATH/bin` (typically `~/go/bin`) is in your PATH.

**Option B: Build from source**

From a checkout of this repository:

```bash
make build
```

This writes the `forgejo-mcp` binary to the repository root.

### 2. Get Your Access Token

1. Log into your Forgejo instance
2. Go to **Settings** → **Applications** → **Access Tokens**
3. Create a new token with the permissions you need (repo, issue, etc.)

### 3. Configure Your AI Assistant

Add this to your MCP configuration file (stdio mode, the most common):

```json
{
  "mcpServers": {
    "forgejo": {
      "command": "forgejo-mcp",
      "args": [
        "--transport", "stdio",
        "--url", "https://your-forgejo-instance.org"
      ],
      "env": {
        "FORGEJO_ACCESS_TOKEN": "<your personal access token>",
        "FORGEJO_USER_AGENT": "forgejo-mcp/1.0.0"
      }
    }
  }
}
```

For streamable HTTP, SSE, multi-tenant use, OAuth resource-server mode, and every option, see [docs/configuration.md](docs/configuration.md).

### 4. Start Using It

Open your MCP-compatible AI assistant and try:

```
List all my repositories
```

## Documentation

- [Configuration](docs/configuration.md): transports, network exposure, authentication, OAuth resource-server mode, every option, and troubleshooting.
- [Tools](docs/reference/tools.md): one row per tool, file output, and lean mode.
- [Toolsets and presets](docs/reference/toolsets.md): load fewer tools.
- [Resources](docs/reference/resources.md): the `forgejo://` resource templates and which clients support them.
- [CLI mode](docs/reference/cli.md): run any tool from the command line without an MCP server.
- [Demos](demos/README.md): copy-pasteable walkthroughs against a live Forgejo instance.
- [OAuth resource-server guides](docs/oauth-resource-server/operator.md): the [operator guide](docs/oauth-resource-server/operator.md) and the [user guide](docs/oauth-resource-server/user.md).
- [Developer guide](DEVELOPER.md): build, architecture, and contribution. Design decisions are in [docs/adr/](docs/adr/).

## Troubleshooting

Set `--debug` or `FORGEJO_DEBUG=true` for detailed logs. If Forgejo or a proxy blocks the default User-Agent, set `FORGEJO_USER_AGENT`. More in [docs/configuration.md](docs/configuration.md#troubleshooting).

## Getting Help

- Report bugs and feature requests through this project's issue tracker.
- **Found a security problem?** Do not use the issue tracker. See [SECURITY.md](SECURITY.md) for how to report it privately.

## Acknowledgements

Thanks to the upstream code contributors whose work this project began from:

- **goern** (Christoph Görn): project creator and maintainer
- **Ronmi Ren**: co-creator; SSE/HTTP transport, issue blocking, CI/CD, logo, Glama spec
- **Tristin Stagg**: user agent configuration
- **Matthew Miller**: logging, `FORGEJO_*` migration, README, URL refactor
- **byteflavour**: notification API, per-request auth, network transport hardening, OAuth resource-server mode
- **jesterret**: pull request reviews and comments
- **appleboy**: custom SSE port, bug fixes
- **ignasgil**: `remove_issue_labels` tool
- **Dmitry Mikushin**: string-encoded number parameter parsing
- **jiriks74**: mcp-go dependency update
- **Tomi Haapaniemi**: `update_pull_request` tool
- **hiifong**: early bug fixes and updates
- **Lunny Xiao**: early contributions
- **techknowlogick**: early contributions
- **yp05327**: early contributions
- **mw75**: owner and org support for repository creation
- **Dax Kelson**: issue comment management
- **Guruprasad Kulkarni**: Arch Linux AUR installation docs
- **Mario Wolff**: contributions
- **Massimo Fraschetti**: contributions
- **David Paul Turley**: repository-scoped tokens, Claude Desktop Extension packaging, issue due dates, cross-repo issue dependencies, and more
- **BrilliantKahn**: plain-text default for `get_file_content`; `list_repo_contents` and `get_repo_tree`
- **Eugene Nesvetaev**: repository, topic, Actions run, package, and commit status tools
- **Marco Pisco**: streaming `file_path` uploads for attachments

## License

Licensed under the GNU General Public License v3.0 or later. See [LICENSE](LICENSE).
