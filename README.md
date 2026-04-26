# mm — Mattermost CLI & MCP server

[![Release](https://img.shields.io/github/v/release/carlosprados/mm?style=flat-square)](https://github.com/carlosprados/mm/releases)
[![Go Reference](https://pkg.go.dev/badge/github.com/carlosprados/mm.svg)](https://pkg.go.dev/github.com/carlosprados/mm)
[![License](https://img.shields.io/github/license/carlosprados/mm?style=flat-square)](LICENSE)

A small, no-nonsense client for [Mattermost](https://mattermost.com/) that ships in
two flavors from a single binary:

- **CLI** — read and post from your terminal without leaving `tmux`.
- **MCP server** — same functionality exposed to AI assistants (Claude Desktop,
  Claude Code, MCP Inspector, …) via the [Model Context Protocol](https://modelcontextprotocol.io/).

> **Parity guarantee.** The CLI and the MCP server are kept at functional
> parity. Every CLI command has a tool / resource / prompt counterpart, and any
> change to one must be mirrored in the other. See the [parity table](#parity-cli--mcp).

Built against Mattermost Server **11.6.x** using the official
[`mattermost/server/public`](https://github.com/mattermost/mattermost) Go client.

---

## Features

- Persistent session via `mm login` — no need to re-export env vars per shell.
- List joined channels (public, private, DMs).
- List team members with their `@username` handle.
- Read the last *N* messages from a channel.
- Send a message to a channel **or** a direct message to a user.
- Resolves user IDs to `@usernames` in batch — no opaque UUIDs.
- MCP server (`mm mcp`) with **5 tools**, **3 resources** and **3 prompts**.

---

## Installation

### From a release (recommended)

Grab the binary for your platform from the
[releases page](https://github.com/carlosprados/mm/releases):

```bash
# Linux / macOS
curl -L https://github.com/carlosprados/mm/releases/latest/download/mm_Linux_x86_64.tar.gz \
  | tar -xz -C /usr/local/bin mm
chmod +x /usr/local/bin/mm
```

### With `go install`

```bash
go install github.com/carlosprados/mm@latest
```

### From source

```bash
git clone https://github.com/carlosprados/mm.git
cd mm
task build           # ./bin/mm
# or: go build -o mm .
```

Requires Go **1.25.8+**.

---

## Getting a Personal Access Token (PAT)

`mm` authenticates with a Mattermost **Personal Access Token**. Steps:

1. **Sign in** to your Mattermost web UI (e.g. `https://chat.example.com`).
2. Click your **avatar** (top-left) → **Profile** → **Security tab**.
3. Scroll to **Personal Access Tokens** → **Create Token**.
4. Give it a description (e.g. `mm CLI`) and **Save**.
5. **Copy the token now** — Mattermost displays it only once. If you lose it,
   you must revoke and create a new one.

> **Don't see the section?** Personal Access Tokens are admin-gated. Ask an
> admin to enable them under
> *System Console → Integrations → Integration Management →
> "Enable Personal Access Tokens" → true*. Your account also needs the
> built-in role `system_user_access_token` (admins typically grant it through
> a System Role).

Once you have the token, [log in](#first-time-setup-mm-login).

---

## First-time setup: `mm login`

```bash
mm login
```

You'll be prompted for the server URL, the token (silent input — no echo) and
the team slug:

```text
Server URL (e.g. https://chat.example.com): https://chat.amplia.es
Personal Access Token:
Team name (slug, optional, press enter to skip): amplia
✓ Logged in as @carlos.prados at https://chat.amplia.es (team: amplia)
Config saved to /home/charlie/.config/mm/config.json
```

`mm` validates the token against the server **before** saving. The config file
is written with mode `0600` at `$XDG_CONFIG_HOME/mm/config.json` (defaults to
`~/.config/mm/config.json`).

You can pass any field as a flag to skip its prompt:

```bash
mm login --url https://chat.amplia.es --team amplia
# only the token is asked interactively
```

To check or remove the active session:

```bash
mm whoami
mm logout
```

### Alternative: environment variables

For CI or one-shot shells, env vars override the saved session:

| Variable    | Description                                                     |
|-------------|-----------------------------------------------------------------|
| `MM_URL`    | Base URL of the server, e.g. `https://chat.example.com`         |
| `MM_TOKEN`  | Personal Access Token                                           |
| `MM_TEAM`   | Team name slug, e.g. `engineering`                              |

```bash
export MM_URL="https://chat.example.com"
export MM_TOKEN="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
export MM_TEAM="engineering"
mm channels
```

Precedence: env vars → saved config → error.

---

## CLI usage

```text
mm [command] [flags]
```

### `mm channels` — list joined channels

```bash
mm channels
```

```text
[public ] town-square
[private] dev-backend
[dm     ] juan.garcia__carlos.prados
```

### `mm users` — list team members

```bash
mm users
```

### `mm read` — read messages from a channel

| Flag              | Default | Description                          |
|-------------------|---------|--------------------------------------|
| `-c, --channel`   | —       | **Required.** Channel name (slug).   |
| `-n, --limit`     | `20`    | Number of messages to fetch.         |

```bash
mm read -c town-square -n 10
```

Output is oldest → newest:

```text
[09:14] @juan.garcia: deploy listo en staging
[09:15] @carlos.prados: probando ahora
[09:18] @maria.lopez: 👍
```

### `mm send` — send a message

| Flag              | Description                                                |
|-------------------|------------------------------------------------------------|
| `-c, --channel`   | Target channel name. Mutually exclusive with `--user`.     |
| `-u, --user`      | Target username for a DM. Mutually exclusive with `--channel`. |
| `-m, --message`   | **Required.** Message body.                                |

```bash
mm send -c dev-backend -m "Deploy listo, revisa logs"
mm send -u juan.garcia  -m "¿Tienes un momento?"
```

### Other commands

| Command       | Purpose                                  |
|---------------|------------------------------------------|
| `mm login`    | Interactive auth + persisted session     |
| `mm logout`   | Remove the saved session                 |
| `mm whoami`   | Show the active session and its source  |
| `mm version`  | Print version, commit and build date     |
| `mm mcp`      | Run as MCP server on stdio               |

---

## MCP server

`mm` is also a Model Context Protocol server. Same auth, same data, exposed
to AI clients. Start it:

```bash
mm mcp
```

It speaks MCP over **stdio**, so it's meant to be invoked by an MCP client,
not run by a human directly.

### Wiring it into Claude Desktop

Add this to `~/Library/Application Support/Claude/claude_desktop_config.json`
(macOS) or `%APPDATA%\Claude\claude_desktop_config.json` (Windows):

```json
{
  "mcpServers": {
    "mattermost": {
      "command": "mm",
      "args": ["mcp"]
    }
  }
}
```

If you didn't run `mm login`, pass credentials via `env`:

```json
{
  "mcpServers": {
    "mattermost": {
      "command": "mm",
      "args": ["mcp"],
      "env": {
        "MM_URL": "https://chat.example.com",
        "MM_TOKEN": "...",
        "MM_TEAM": "engineering"
      }
    }
  }
}
```

> Recommended: do `mm login` once on your machine and let the MCP server reuse
> the saved session — no secrets in the JSON file.

### Wiring it into Claude Code

```bash
claude mcp add mattermost -- mm mcp
```

### Inspecting / smoke-testing

```bash
npx @modelcontextprotocol/inspector mm mcp
```

### Parity (CLI ↔ MCP)

| CLI              | MCP tool       | MCP resource                                | MCP prompt                                                |
|------------------|----------------|---------------------------------------------|-----------------------------------------------------------|
| `mm channels`    | `list_channels`| `mm://team/channels`                        | —                                                         |
| `mm users`       | `list_users`   | `mm://team/users`                           | —                                                         |
| `mm read`        | `read_channel` | `mm://channel/{name}/messages?limit={n}`    | feeds `summarize_channel`, `draft_reply`, `daily_digest`  |
| `mm send`        | `send_message` | —                                           | —                                                         |
| `mm whoami`      | `whoami`       | —                                           | —                                                         |
| `mm login/logout`| _host-side only_ — MCP reuses the saved session | — | —                                                |

**Tools** — explicit RPC actions (write side effects, parameterized reads).
**Resources** — pull-model addressable data; some clients prefer them over tools.
**Prompts** — templates that hydrate themselves with live channel data and
return ready-to-reason context.

Available prompts:

- `summarize_channel(channel, limit?)` — decisions, blockers, action items.
- `draft_reply(channel, intent, limit?)` — draft a reply matching tone.
- `daily_digest(channels, limit?)` — multi-channel digest.

---

## Development

This project uses [Task](https://taskfile.dev) (`task` binary):

```bash
task            # list tasks
task tidy       # go mod tidy
task fmt        # gofmt -w .
task vet        # go vet ./...
task build      # ./bin/mm
task install    # go install
task run -- channels
task mcp        # run MCP server (for `mcp inspector`)
task test
task release:check
task release:snapshot
task clean
```

If you don't have Task installed, the underlying commands are trivial:

```bash
go build -o bin/mm .
go run . channels
```

### Project layout

```
mm/
├── Taskfile.yml
├── main.go
├── cmd/                  — Cobra commands (CLI surface)
│   ├── root.go
│   ├── login.go
│   ├── logout.go
│   ├── whoami.go
│   ├── channels.go
│   ├── read.go
│   ├── send.go
│   ├── users.go
│   ├── mcp.go
│   └── version.go
└── internal/
    ├── client/           — Mattermost API wrapper
    │   └── mattermost.go
    ├── config/           — Persisted session (XDG, 0600)
    │   └── config.go
    └── mcp/              — MCP server (parity with CLI)
        ├── server.go
        ├── tools.go
        ├── resources.go
        └── prompts.go
```

### Branching model

git-flow:

1. Branch from `develop`: `git checkout -b feat/<name> develop`
2. PR into `develop`.
3. Releases: PR `develop → main`, tag on `main` → GoReleaser publishes.

### Conventions

- Code, comments and identifiers in English.
- Errors wrapped with `fmt.Errorf("context: %w", err)`.
- No global state beyond Cobra flag vars.
- **CLI ↔ MCP parity is mandatory.** New CLI commands MUST add the MCP
  counterpart (and vice versa) in the same PR.

---

## Releases

Tagged releases are built and published automatically by
[GoReleaser](https://goreleaser.com/) via GitHub Actions:

```bash
git tag v0.2.0
git push origin v0.2.0
```

The workflow produces:

- Binaries for Linux, macOS and Windows (amd64 + arm64).
- `tar.gz` / `zip` archives.
- `checksums.txt` with SHA-256 sums.
- A GitHub Release with an auto-generated changelog.

See [`.goreleaser.yaml`](.goreleaser.yaml) for the full configuration.

---

## License

[MIT](LICENSE) © Carlos Prados
