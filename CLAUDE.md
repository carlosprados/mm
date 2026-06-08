# mm — Mattermost CLI · Claude Code Handoff

## Context

Go CLI + MCP server to interact with a self-hosted Mattermost Server 11.6.1
at `https://chat.amplia.es`. Single-binary, Cobra-based, PAT auth, persistent
session via `mm login`. Built as a dual-mode tool: same functionality is
exposed via the CLI (for humans) and via MCP (for AI clients like Claude
Desktop / Claude Code).

## Dependencies

```
github.com/mattermost/mattermost/server/public/model     ← official MM Go client (v11)
github.com/modelcontextprotocol/go-sdk                   ← MCP server (v1.5.0)
github.com/spf13/cobra
golang.org/x/term                                        ← password prompt for `mm login`
```

**Go toolchain:** 1.25.8+ (required by `mattermost/server/public` v0.3.1).

Run after cloning:
```bash
task tidy   # or: go mod tidy
task build  # or: go build -o bin/mm .
```

## Authentication

Two ways, with this precedence:

1. **Saved session** — `mm login` writes `$XDG_CONFIG_HOME/mm/config.json`
   (mode `0600`). Default for human users.
2. **Environment variables** — `MM_URL`, `MM_TOKEN`, `MM_TEAM`. Override the
   saved session, useful for CI / one-off shells.

Generate a PAT in Mattermost → **Profile → Security → Personal Access Tokens → Create**.

## CLI ↔ MCP parity (LOAD-BEARING RULE)

The CLI and the MCP server expose the **same functionality**. Whenever you
add, modify or remove a CLI command, you must apply the equivalent change to
the MCP server (and vice versa). PRs that break parity should be rejected.

Mapping table (keep in sync with the code):

| CLI command            | MCP tool          | MCP resource(s)                              | MCP prompt(s)                                   |
|------------------------|-------------------|----------------------------------------------|-------------------------------------------------|
| `mm channels`          | `list_channels`   | `mm://team/channels`                         | —                                               |
| `mm users`             | `list_users`      | `mm://team/users`                            | —                                               |
| `mm read -c X -n N`    | `read_channel`    | `mm://channel/{name}/messages?limit=N`       | feeds `summarize_channel`, `draft_reply`, `daily_digest` |
| `mm send …`            | `send_message`    | —                                            | —                                               |
| `mm alias add/rm/list` | `manage_alias`    | —                                            | —                                               |
| `mm whoami`            | `whoami`          | —                                            | —                                               |
| `mm login` / `logout`  | _intentionally not exposed_ — auth is host-side, the MCP server reuses the saved session | — | — |

Tools = explicit RPC actions. Resources = pull-model addressable data.
Prompts = templates that hydrate themselves with live data and return ready-to-reason context.

When in doubt about which surface to use:
- **Side effect or write** → tool only.
- **Read with parameters** → tool + resource template (compatibility).
- **Read of a fixed dataset** → resource (and optionally a tool wrapper).
- **Common workflow that combines reads + framing** → prompt.

## Project layout

```
mm/
├── Taskfile.yml
├── main.go
├── cmd/
│   ├── root.go        — Cobra root, Execute()
│   ├── channels.go    — `mm channels`
│   ├── read.go        — `mm read -c <channel> [-n <limit>]`
│   ├── send.go        — `mm send [-c <channel>|-u <username>] -m <message>`
│   ├── users.go       — `mm users`
│   ├── alias.go       — `mm alias add|rm|list` — short handles → usernames
│   ├── login.go       — `mm login` interactive auth + persisted session
│   ├── logout.go      — `mm logout`
│   ├── whoami.go      — `mm whoami`
│   ├── mcp.go         — `mm mcp` → MCP server on stdio
│   └── version.go     — `mm version`
└── internal/
    ├── alias/
    │   └── alias.go       — alias→username store (aliases.json, 0644), Resolve()
    ├── client/
    │   └── mattermost.go  — MM struct, New(), env+config precedence
    ├── config/
    │   └── config.go      — XDG-aware credential persistence (0600)
    └── mcp/
        ├── server.go      — wires up tools/resources/prompts
        ├── tools.go       — 6 tools
        ├── resources.go   — 3 resources (1 fixed + 2 templated)
        └── prompts.go     — 3 prompts
```

## Conventions

- Code and comments in English. Conversation in Spanish.
- Errors wrapped with `fmt.Errorf("context: %w", err)`.
- No global state beyond Cobra flag vars.
- Do not auto-push or add co-author attributions.
- Branching: feature branches off `develop`; PR to `develop`; release-time PR
  to `main` + tag → GoReleaser publishes.
