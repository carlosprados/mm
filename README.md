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
- Edit your own messages from the CLI, the TUI (`↑`) or MCP.
- Schedule messages for later delivery (CLI, TUI `ctrl+t`, MCP); delivered by the TUI while it runs.
- Configurable aliases: DM a colleague by a short handle (`luis` → `luisdavid.francisco`).
- Interactive TUI (`mm tui`) built on Bubble Tea, with Markdown rendering and an emoji picker.
- MCP server (`mm mcp`) with **9 tools**, **3 resources** and **3 prompts**.

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
| `-u, --user`      | Target username **or alias** for a DM. Mutually exclusive with `--channel`. |
| `-m, --message`   | **Required.** Message body.                                |

```bash
mm send -c dev-backend -m "Deploy listo, revisa logs"
mm send -u juan.garcia  -m "¿Tienes un momento?"
mm send -u luis         -m "¿Tienes un momento?"   # luis is an alias
```

### `mm edit` — edit one of your messages

Edits your most recent message in a channel or DM, or a specific post with
`--post`. You can only edit your own messages.

| Flag              | Description                                                |
|-------------------|------------------------------------------------------------|
| `-c, --channel`   | Target channel. Edits your last message there.             |
| `-u, --user`      | Target username or alias. Edits your last DM message there.|
| `--post`          | Edit a specific post by ID instead of your last message.   |
| `-m, --message`   | **Required.** New message body.                            |

```bash
mm edit -c dev-backend -m "Deploy listo (corregido)"
mm edit -u luis        -m "Perdón, quería decir mañana"
```

### `mm schedule` — send messages later

> This server has no scheduled-posts license, so delivery is done by **mm
> itself**: scheduled messages are stored locally
> (`$XDG_CONFIG_HOME/mm/scheduled.json`) and delivered by the **TUI while it is
> running**. If the TUI is not running at the due time, the message is sent the
> next time the TUI starts (overdue messages are caught up). `mm schedule add`
> from the CLI only records the message.

```bash
mm schedule add -c dev-backend -m "Buenos días, recordad la demo" --at "2026-06-09 09:00"
mm schedule add -u luis -m "Te llamo en un rato" --at "+2h"
mm schedule list
mm schedule rm <id>
```

`--at` accepts `"2006-01-02 15:04"` (local), RFC3339, `"15:04"` (today), or a
relative `"+2h"` / `"+90m"`.

### `mm alias` — short handles for colleagues

Map a short handle to a canonical username so you can DM `luisdavid.francisco`
by typing `luis` (or `luisete`). Several aliases may point to the same user.
Aliases are resolved by `mm send -u`, by the TUI and by the MCP `send_message`
tool. They are stored in `$XDG_CONFIG_HOME/mm/aliases.json` (mode `0644`, no
secrets).

```bash
mm alias add luis luisdavid.francisco
mm alias add luisete luisdavid.francisco
mm alias list
mm alias rm luisete
```

### Other commands

| Command       | Purpose                                  |
|---------------|------------------------------------------|
| `mm login`    | Interactive auth + persisted session     |
| `mm logout`   | Remove the saved session                 |
| `mm whoami`   | Show the active session and its source  |
| `mm version`  | Print version, commit and build date     |
| `mm mcp`      | Run as MCP server on stdio               |
| `mm tui`      | Launch the interactive terminal UI       |

---

## TUI (`mm tui`)

A full-screen terminal client: a channel/DM sidebar on the left, a
Markdown-rendered message pane and a composer on the right. DMs are labelled by
the colleague's alias when one is configured. Same auth as the CLI; the active
channel is refreshed by polling.

**Unread first.** The sidebar prioritizes channels/DMs with messages you
haven't read: they sort to the top (most recent first) with a `●` bullet and an
`(N)` mention count. Opening a channel marks it read (server-side, so it also
clears on web/mobile). The list refreshes itself periodically.

**Scroll & copy.** `tab` to the message pane, then `j`/`k` / `pgup`/`pgdn` to
scroll; your position is kept across background refreshes (it only snaps to the
bottom if you were already there). Press `y` to copy any message's Markdown
source to the clipboard — handy for code blocks and formatted text alike.
Clipboard needs `xclip`/`xsel` (X11) or `wl-copy` (Wayland).

| Key             | Action                                                       |
|-----------------|--------------------------------------------------------------|
| `tab`           | Cycle focus: sidebar → messages → composer                   |
| `j` / `k`       | Move within the focused pane (scroll the message pane when focused) |
| `y`             | On the message pane: open the copy picker — pick a message, `enter`/`y` copies its **Markdown source** to the clipboard |
| `/`             | Filter the sidebar (matches alias and @handle)               |
| `enter`         | Open the selected channel (marks it read; focus → composer)  |
| `a`             | On a selected DM: assign an alias to that colleague          |
| `s`             | Open the scheduled-messages viewer (then `x` cancels one)    |
| `ctrl+s`        | Send the composed message                                    |
| `ctrl+t`        | Schedule the composed message (prompts for a delivery time)  |
| `:` + text      | Emoji picker — fuzzy search, `↑`/`↓` to choose, `enter`/`tab` to insert |
| `↑` / `↓`       | In the composer: walk back/forward through **your** messages to edit them; `↓` past the newest restores your draft |
| `esc`           | Close the picker/viewer / cancel an edit (restores the draft) / back to sidebar |
| `r`             | Refresh                                                      |
| `q` / `ctrl+c`  | Quit (`ctrl+c` always; `q` is text while composing/filtering)|

- Editing your own messages (the `↑` flow) maps to the same capability as
  `mm edit` and the `edit_message` MCP tool.
- Assigning an alias with `a` writes the same `aliases.json` used by
  `mm alias` and the `manage_alias` MCP tool — the three surfaces stay in sync.

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
| `mm edit`        | `edit_message` | —                                           | —                                                         |
| `mm schedule add`| `schedule_message` | —                                       | —                                                         |
| `mm schedule list/rm` | `manage_scheduled` | —                                  | —                                                         |
| `mm alias`       | `manage_alias` | —                                           | —                                                         |
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
│   ├── edit.go
│   ├── schedule.go
│   ├── alias.go
│   ├── users.go
│   ├── mcp.go
│   ├── tui.go
│   └── version.go
└── internal/
    ├── alias/            — alias→username store (XDG, 0644)
    │   └── alias.go
    ├── client/           — Mattermost API wrapper
    │   ├── mattermost.go
    │   └── messaging.go  — Target, Send, EditPost (shared by CLI/TUI/MCP)
    ├── config/           — Persisted session (XDG, 0600)
    │   └── config.go
    ├── schedule/         — client-side scheduled messages (XDG, 0600)
    │   ├── schedule.go   — store: Add/Remove/Due/Sorted
    │   └── time.go       — ParseTime
    ├── mcp/              — MCP server (parity with CLI)
    │   ├── server.go
    │   ├── tools.go
    │   ├── resources.go
    │   └── prompts.go
    └── tui/              — interactive terminal UI (Bubble Tea)
        ├── model.go
        ├── update.go
        ├── view.go
        ├── keys.go
        ├── messages.go
        ├── emoji.go
        └── styles.go
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
