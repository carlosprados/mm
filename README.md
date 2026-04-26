# mm — Mattermost CLI

[![Release](https://img.shields.io/github/v/release/carlosprados/mm?style=flat-square)](https://github.com/carlosprados/mm/releases)
[![Go Reference](https://pkg.go.dev/badge/github.com/carlosprados/mm.svg)](https://pkg.go.dev/github.com/carlosprados/mm)
[![License](https://img.shields.io/github/license/carlosprados/mm?style=flat-square)](LICENSE)

A small, no-nonsense command-line client for [Mattermost](https://mattermost.com/).
Single static binary, Personal Access Token auth, sane defaults — just enough to read
and post from your terminal without leaving `tmux`.

Built against Mattermost Server **11.6.x** using the official
[`mattermost/server/public`](https://github.com/mattermost/mattermost) Go client.

---

## Features

- List joined channels (public, private, DMs).
- List team members with their `@username` handle.
- Read the last *N* messages from a channel.
- Send a message to a channel **or** a direct message to a user.
- Resolves user IDs to `@usernames` in batch — no opaque UUIDs in your terminal.

---

## Installation

### From a release (recommended)

Grab the binary for your platform from the
[releases page](https://github.com/carlosprados/mm/releases) and drop it in your `$PATH`:

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
go build -o mm .
```

Requires Go **1.25.8+**.

---

## Configuration

`mm` reads credentials from environment variables. Drop these in your shell profile or
load them with [`direnv`](https://direnv.net/):

| Variable    | Required | Description                                         |
|-------------|----------|-----------------------------------------------------|
| `MM_URL`    | yes      | Base URL of the server, e.g. `https://chat.example.com` |
| `MM_TOKEN`  | yes      | Personal Access Token (see below)                   |
| `MM_TEAM`   | yes\*    | Team name (slug), e.g. `engineering`                |

> \* `MM_TEAM` is required for any command that operates on a team (channels, users, read, send to channel).

### Generating a Personal Access Token

1. In Mattermost: **Profile → Security → Personal Access Tokens → Create**.
2. Copy the token (you only see it once).
3. Export it:

```bash
export MM_URL="https://chat.example.com"
export MM_TOKEN="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
export MM_TEAM="engineering"
```

If your account does not have permission to create PATs, ask an admin to enable
**Personal Access Tokens** under *System Console → Integrations → Integration Management*.

---

## Usage

```text
mm [command] [flags]
```

### `mm channels` — list joined channels

```bash
mm channels
```

```text
[public ] town-square
[public ] off-topic
[private] dev-backend
[dm     ] juan.garcia__carlos.prados
```

### `mm users` — list team members

```bash
mm users
```

```text
@juan.garcia          Juan García
@carlos.prados        Carlos Prados
@maria.lopez          María López
```

### `mm read` — read messages from a channel

| Flag              | Default | Description                          |
|-------------------|---------|--------------------------------------|
| `-c, --channel`   | —       | **Required.** Channel name (slug).   |
| `-n, --limit`     | `20`    | Number of messages to fetch.         |

```bash
mm read -c town-square -n 10
```

```text
[09:14] @juan.garcia: deploy listo en staging
[09:15] @carlos.prados: probando ahora
[09:18] @maria.lopez: 👍
```

Messages are printed oldest → newest.

### `mm send` — send a message

| Flag              | Default | Description                                                |
|-------------------|---------|------------------------------------------------------------|
| `-c, --channel`   | —       | Target channel name. Mutually exclusive with `--user`.     |
| `-u, --user`      | —       | Target username for a DM. Mutually exclusive with `--channel`. |
| `-m, --message`   | —       | **Required.** Message body.                                |

Send to a channel:

```bash
mm send -c dev-backend -m "Deploy listo, revisa logs"
```

Send a direct message:

```bash
mm send -u juan.garcia -m "¿Tienes un momento?"
```

---

## Examples

```bash
# Quick standup digest
mm read -c standup -n 5

# Notify the team after a deploy
mm send -c dev-backend -m "Release v1.4.2 desplegada en producción"

# Ping a colleague
mm send -u maria.lopez -m "Reunión a las 16:00"

# Find that one channel you joined six months ago
mm channels | grep -i security
```

---

## Releases

Tagged releases are built and published automatically by
[GoReleaser](https://goreleaser.com/) via GitHub Actions:

```bash
git tag v0.1.0
git push origin v0.1.0
```

The workflow produces:

- Binaries for Linux, macOS and Windows (amd64 + arm64).
- `tar.gz` / `zip` archives.
- `checksums.txt` with SHA-256 sums.
- A GitHub Release with auto-generated changelog.

See [`.goreleaser.yaml`](.goreleaser.yaml) for the full configuration.

---

## Development

```bash
go mod tidy
go build -o mm .
go vet ./...
```

Project layout:

```
mm/
├── main.go
├── cmd/                — Cobra commands
│   ├── root.go
│   ├── channels.go
│   ├── read.go
│   ├── send.go
│   ├── users.go
│   └── version.go
└── internal/client/    — Mattermost client wrapper
    └── mattermost.go
```

### Conventions

- Code, comments and identifiers in English.
- Errors wrapped with `fmt.Errorf("context: %w", err)`.
- No global state beyond Cobra flag vars.
- One responsibility per command file.

---

## License

[MIT](LICENSE) © Carlos Prados
