<!-- SPDX-License-Identifier: GPL-3.0-or-later -->
# Contributors (inherited from upstream)


forgejo-mcp is shaped by everyone who files issues, writes code, reviews PRs, and pushes the project forward. Thank you all. 🙏

### Code contributors

| Contributor | Highlights |
|-------------|------------|
| [goern](https://codeberg.org/goern) (Christoph Görn) | Project creator and maintainer |
| Ronmi Ren | Co-creator; SSE/HTTP transport, issue blocking, CI/CD improvements, logo, Glama spec |
| [twstagg](https://codeberg.org/twstagg) (Tristin Stagg) | User agent configuration support (PR #89) |
| [mattdm](https://codeberg.org/mattdm) (Matthew Miller) | Logging improvements, FORGEJO_* migration, README, URL refactor |
| [byteflavour](https://codeberg.org/byteflavour) | `check_notifications` + full notification management API (PR #84, #86); stateless per-request auth for HTTP/SSE transports (PR #138); NixOS installation docs (PR #146); network transport hardening — loopback-by-default bind, `Host`/`Origin` checks, per-request credential (PR #545, landed as #573; #586, #585, #589); **OAuth resource-server mode** — inbound JWT validation and outbound Forgejo 16 Authorized Integration signing, with operator and user guides and 40 anchored demo scenarios (PR #584, researched in #582); dev-env `FORGEJO_MCP_EXEC` (PR #580); feature requests #80, #85 |
| [jesterret](https://codeberg.org/jesterret) | Pull request reviews and comments support (PR #51) |
| [appleboy](https://codeberg.org/appleboy) | Custom SSE port support, bug fixes |
| [ignasgil](https://codeberg.org/ignasgil) | `remove_issue_labels` tool (PR #96) |
| [dmikushin](https://codeberg.org/dmikushin) (Dmitry Mikushin) | Fix string-encoded number parameter parsing from MCP clients (PR #93) |
| [jiriks74](https://codeberg.org/jiriks74) | mcp-go v0.44.0 dependency update (PR #90) |
| [th](https://codeberg.org/th) (Tomi Haapaniemi) | `update_pull_request` tool |
| [hiifong](https://codeberg.org/hiifong) | Early bug fixes and updates |
| [Lunny Xiao](https://codeberg.org/lunny) | Early contributions |
| [techknowlogick](https://codeberg.org/techknowlogick) | Early contributions |
| [yp05327](https://codeberg.org/yp05327) | Early contributions |
| [mw75](https://codeberg.org/mw75) | Owner/org support for repo creation (PR #18) |
| [Dax Kelson](https://codeberg.org/dkelson) | Issue comment management (PR #34) |
| [Guruprasad Kulkarni](https://codeberg.org/comdotlinux) | Arch Linux AUR installation docs (PR #69) |
| [Mario Wolff](https://codeberg.org/mariowolff) | Contributions |
| [Massimo Fraschetti](https://codeberg.org/fraschetti) | Contributions |
| [synath](https://codeberg.org/synath) (David Paul Turley) | Repository-scoped token support via `ServerVersion` probe (PR #112); merge status-code check (PR #113); Claude Desktop Extension (.mcpb) packaging (PR #118, #123); issue `due_date` + server-side sort (PR #483); bounded issue-list and comment-thread resources (PR #487); `total_count` on paginated envelopes from `X-Total-Count` (PR #507); `create_*_attachment` timeout hardening (PR #534, #536); cross-repo issue dependencies (PR #535) |
| [BrilliantKahn](https://codeberg.org/BrilliantKahn) | `get_file_content` plain-text default (PR #116); `list_repo_contents` and `get_repo_tree` tools (PR #117). **First-ever open source contribution** — welcome aboard! 🎉 |
| [nesvet](https://git.b4mad.industries/nesvet) (Eugene Nesvetaev) | `get_repo`/`edit_repo` (PR #527); repository topic tools (PR #528); Actions run cancel/delete and run artifacts (PR #533); `get_commit_statuses` (PR #542); package list/get/delete/files tools (PR #543); label names accepted on issue create, assignment and replace (PR #591) |
| [pisco](https://git.b4mad.industries/pisco) (Marco Pisco) | `file_path` uploads for issue, comment, and release attachments, with streaming multipart so large release assets no longer round-trip through base64 (PR #481) |

### Community contributors

Issue reporters and discussion participants who shaped the direction of the project:

| Contributor | Contributions |
|-------------|--------------|
| [byteflavour](https://codeberg.org/byteflavour) | Filed #80 (milestone/label discovery), #85 (notification API proposal); active reviewer in discussions |
| [choucavalier](https://codeberg.org/choucavalier) | Filed #82 (fix skill), #70 (macOS arm64 releases), #62 (binary releases & mise support) |
| [MalcolmMielle](https://codeberg.org/MalcolmMielle) | Filed #59 (PR review tools — since implemented) |
| [redbeard](https://codeberg.org/redbeard) | Filed #60 (Actions support — since implemented) |
| [c6sepl6p](https://codeberg.org/c6sepl6p) | Filed #72 (base64 encoding), #54 (merge pull request — since implemented) |
| [malik](https://codeberg.org/malik) | Filed #73 (version flag), #47 (Nix build fix) |
| [a2800276](https://codeberg.org/a2800276) | Filed #74 (OpenAI compatibility) |
| [simenandre](https://codeberg.org/simenandre) | Filed #49 (go install support) |
| [BasdP](https://codeberg.org/BasdP) | Filed #42 (Projects support) |
| [BoBeR182](https://codeberg.org/BoBeR182) | Filed #32 (wiki support) |
| [ignasgil](https://codeberg.org/ignasgil) | Filed #95 (`remove_issue_labels` feature request) |
| [Vokuar](https://codeberg.org/Vokuar) | Filed #99 (streamable HTTP transport support) |
| [janbaer](https://codeberg.org/janbaer) | Filed #98 (reply to review comment) |
| [fraschm98](https://codeberg.org/fraschm98) | Early issue reports |
| [heathen711](https://codeberg.org/heathen711) | Filed #106 (issue/comment attachments — since implemented); shaped the 1 MiB inline cap + `browser_download_url` fall-through design |
| [decarvalhoaa](https://git.b4mad.industries/decarvalhoaa) (Antonio De Carvalho) | Filed #593 (server-side response size cap) from a real context-window overflow behind Open WebUI, with the measurement that prompted the `list_repo_pull_requests` payload work in #596 |
| [chris420](https://git.b4mad.industries/chris420) (Chris Oloff) | Filed #452 (org-wide issue search — since implemented as `search_issues`); design review on PR #458 that replaced the next-page probe with instance-ceiling enforcement, and caught the response envelope misreporting its own `limit` |

### Cyborg contributors

This project also received contributions from AI coding agents — submitted as regular PRs, reviewed by humans:

| Agent | Role | Contributions |
|-------|------|---------------|
| [brenner-axiom](https://codeberg.org/brenner-axiom) (b4-dev, B4arena) | AI dev agent | Organization management tools (PR #94); showboat demos (PR #97); `list_repo_milestones`, `list_repo_labels` tools (PR #83); race condition fix (PR #78); contributors docs (PR #87, #88); filed #76; code reviews |
| opencode | AI dev agent | Pull request reviews and comments support (PR #51) |
| claude-code | AI dev agent | `get_file_content` plain-text default and `list_repo_contents`/`get_repo_tree` tools, paired with [BrilliantKahn](https://codeberg.org/BrilliantKahn) (PR #116, #117) |
| b4mad-release-agent | Release automation | Automated changelog and release tagging |
| the #B4mad Renovate bot | Dependency updates | Automated dependency upgrades |

Want to contribute? Open an issue or pull request — all are welcome.

