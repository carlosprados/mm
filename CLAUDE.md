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
| `mm edit …`            | `edit_message`    | —                                            | —                                               |
| `mm schedule add`      | `schedule_message`| —                                            | —                                               |
| `mm schedule list/rm`  | `manage_scheduled`| —                                            | —                                               |
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
│   ├── edit.go        — `mm edit [-c <channel>|-u <username>] [--post <id>] -m <message>`
│   ├── schedule.go    — `mm schedule add|list|rm` — server-side scheduled posts
│   ├── users.go       — `mm users`
│   ├── alias.go       — `mm alias add|rm|list` — short handles → usernames
│   ├── login.go       — `mm login` interactive auth + persisted session
│   ├── logout.go      — `mm logout`
│   ├── whoami.go      — `mm whoami`
│   ├── mcp.go         — `mm mcp` → MCP server on stdio
│   ├── tui.go         — `mm tui` → interactive terminal UI (Bubble Tea)
│   └── version.go     — `mm version`
└── internal/
    ├── alias/
    │   └── alias.go       — alias→username store (aliases.json, 0644), Resolve()
    ├── client/
    │   ├── mattermost.go  — MM struct, New(), env+config precedence
    │   └── messaging.go   — Target, ResolveChannelID, Send, EditPost (shared by CLI/TUI/MCP)
    ├── config/
    │   └── config.go      — XDG-aware credential persistence (0600)
    ├── schedule/
    │   ├── schedule.go    — client-side scheduled-message store (scheduled.json, 0600)
    │   └── time.go        — ParseTime (shared by CLI/TUI/MCP)
    ├── mcp/
    │   ├── server.go      — wires up tools/resources/prompts
    │   ├── tools.go       — 9 tools
    │   ├── resources.go   — 3 resources (1 fixed + 2 templated)
    │   └── prompts.go     — 3 prompts
    └── tui/               — interactive terminal UI (Bubble Tea)
        ├── model.go       — root model, focus, channelItem, edit/alias/emoji state
        ├── update.go      — Update loop + async tea.Cmds (polling), layout
        ├── view.go        — lipgloss layout (sidebar + messages + emoji popup + composer + footer)
        ├── keys.go        — app-level keymap
        ├── messages.go    — tea.Msg types
        ├── emoji.go       — emoji dataset + ":query" search (kyokomi/emoji)
        └── styles.go      — lipgloss styles
```

## TUI (`mm tui`)

A third surface alongside the CLI and MCP, built on Bubble Tea + bubbles +
lipgloss, with Markdown rendered by glamour. It reuses `client.MM` and the
shared messaging service; it does **not** introduce its own Mattermost logic.
Auth is host-side (saved session / env), like `mm mcp`, so it has no MCP
counterpart. Updates are **real-time over WebSocket** (`client.ConnectWS` /
`model.WebSocketClient`): `posted`/`post_edited`/`post_deleted` events refetch
the active channel and bubble unread in the sidebar. A forwarder goroutine
drains the ping/response channels; on disconnect the TUI reconnects with a
short backoff. The 20s schedule tick doubles as a safety refresh while the
socket is down.

TUI extras that stay leveled with the other surfaces:
- **Edit** your own messages with `↑` (same as `mm edit` / `edit_message`).
- **Alias** a DM's user with `a` — writes the same `aliases.json` as
  `mm alias` / `manage_alias`.
- **Emoji picker** on a trailing `:query` in the composer (fuzzy search via
  `kyokomi/emoji`); inserts the unicode glyph.
- **Unread-first sidebar**: channels/DMs with unread messages sort to the top
  (`●` bullet + mention count), via `client.ChannelMembers` (LastViewedAt vs
  channel LastPostAt). Opening a channel calls `client.MarkChannelRead`
  (`ViewChannel`) — a server-side side effect that also clears unread on
  web/mobile. The sidebar reloads on the schedule tick (selection preserved).
- **Scheduled-messages viewer**: `s` from the sidebar lists pending scheduled
  messages from `internal/schedule`; `x` cancels the selected one.
- **Scroll & copy**: the message pane preserves scroll position across polling
  reloads (only `GotoBottom` when already at bottom). `y` opens a copy picker
  that writes a message's Markdown source to the clipboard via
  `github.com/atotto/clipboard` (xclip/xsel/wl-copy backends).
- **Inline images**: `i` opens an image-attachment picker (file infos via
  `GetFileInfosForPost`, filtered by `image/*` MimeType). Selecting one downloads
  it (`GetFile`) to a temp file and renders it with `chafa` via
  `tea.ExecProcess` — the TUI is suspended so chafa's sixel/kitty/iterm output
  isn't clobbered by the renderer; the temp file is removed on return. External
  binary dependency: `chafa`.
- **Schedule** the composed message with `ctrl+t` (same store as `mm schedule` /
  `schedule_message`). This server has no scheduled-posts license, so delivery
  is **client-side**: the TUI's delivery loop (`scheduleTickCmd`) sends due
  items from `internal/schedule` while it runs (overdue items are caught up on
  start). The CLI/MCP only record into the store.
`internal/tui/emoji.go` holds the emoji dataset + search; it is a TUI-only
input affordance (the resulting text is sent like any other), so it needs no
CLI/MCP counterpart.

## Conventions

- Code and comments in English. Conversation in Spanish.
- Errors wrapped with `fmt.Errorf("context: %w", err)`.
- No global state beyond Cobra flag vars.
- Do not auto-push or add co-author attributions.
- Branching: feature branches off `develop`; PR to `develop`; release-time PR
  to `main` + tag → GoReleaser publishes.
