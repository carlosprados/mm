# mm — Mattermost CLI · Claude Code Handoff

## Context

Go CLI to interact with a self-hosted Mattermost Server 11.6.1 at `https://chat.amplia.es`.
Single-binary, Cobra-based, PAT auth. Follows the same conventions as the `jira` CLI already
in this workspace.

## Dependencies

```
github.com/mattermost/mattermost/server/public/model  ← official Mattermost Go client (v11 compatible)
github.com/spf13/cobra
```

**Go toolchain:** 1.25.8+ (required by `mattermost/server/public` v0.3.1).

Run after cloning:
```bash
go mod tidy
```

## Environment variables (required at runtime)

| Var        | Description                                 |
|------------|---------------------------------------------|
| `MM_URL`   | Server URL, e.g. `https://chat.amplia.es`   |
| `MM_TOKEN` | Personal Access Token (PAT)                 |
| `MM_TEAM`  | Team name, e.g. `amplia`                    |

Generate a PAT in Mattermost → **Profile → Security → Personal Access Tokens → Create**.

## Project layout

```
mm/
├── main.go
├── cmd/
│   ├── root.go       — Cobra root, Execute()
│   ├── channels.go   — `mm channels`: list joined channels
│   ├── read.go       — `mm read -c <channel> [-n <limit>]`: read messages
│   ├── send.go       — `mm send [-c <channel>|-u <username>] -m <message>`
│   └── users.go      — `mm users`: list team members
└── internal/
    └── client/
        └── mattermost.go  — MM struct, New(), GetDirectChannelWith(), ResolveUsernames()
```

## Commands

```bash
mm channels                              # list joined channels
mm users                                 # list team members with @username
mm read -c general -n 10                 # last 10 messages in #general
mm send -c dev-backend -m "Deploy listo" # send to a channel
mm send -u juan.garcia -m "¿Tienes un momento?" # send DM
```

## Status

All commands implemented and compiling. `ResolveUsernames` resolves a batch of user IDs
to `@username` in a single API call (dedupe → `GetUsersByIds` → fallback to raw ID).

All client methods (`New`, `GetDirectChannelWith`, `ResolveUsernames`) take
`context.Context` as the first argument; cobra commands propagate `cmd.Context()`.

### Functional verification (pending)

Set `MM_URL`, `MM_TOKEN`, `MM_TEAM` and run:

```bash
mm read -c general -n 5  # output must show @username, not raw UUIDs
```

## Conventions

- Code and comments in English.
- Errors wrapped with `fmt.Errorf("context: %w", err)`.
- No global state beyond Cobra flag vars.
- Do not add dependencies beyond those already in go.mod.
- Do not auto-push or add co-author attributions.
