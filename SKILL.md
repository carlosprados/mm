---
name: mm
description: Read and write Mattermost from the shell with the `mm` CLI — read a channel or a DM, see what is unread across every channel (mentions first, without marking anything read), send messages and DMs with file attachments, edit or delete your own messages, download the files people sent, schedule messages for later, list channels and team members, and manage short aliases for usernames. Also runs as an MCP server (`mm mcp`) with the same functions. Load whenever the task mentions Mattermost, a chat message, a DM or "mensaje directo", a channel like #town-square, a Mattermost permalink (https://host/team/pl/<id>), "qué me he perdido" / catching up on chat, sending or downloading a file through the chat, or writing to a colleague. Use INSTEAD of calling the Mattermost REST API by hand.
---

# mm — Mattermost from the shell

`mm` talks to a Mattermost server as the user, with the user's own Personal
Access Token. Everything it sends comes **from the user's account** and reaches
**real people**.

```sh
mm --help · mm <cmd> --help     # every command has examples
mm skill show                   # this document
mm whoami                       # who you are acting as, on which server and team
```

Credentials come from `mm login` (saved in `$XDG_CONFIG_HOME/mm/config.json`)
or from `MM_URL`, `MM_TOKEN`, `MM_TEAM`. Never run `mm login` yourself, never
ask for a token in the chat and never edit the config: if a command fails with
`missing credentials` or `auth failed`, tell the user to run `mm login`.

## Rules before writing anything

1. **Confirm the recipient first.** `-u` takes a username or an alias.
   A wrong but existing handle delivers the message to the wrong person without
   any error. Check with `mm users` (every team member, `@username  Full Name`)
   or `mm alias list` before the first message to someone.
2. **Show the user the message before you send it**, unless they dictated it word
   for word or told you to send without asking. After sending, report literally
   what was sent and to whom.
3. **Sending cannot be undone.** `mm edit` and `mm delete` change the post, but
   someone may have read it, and push notifications already went out.
4. **`mm delete` is irreversible** and removes the message, attachments
   included, for everyone. Read the message first (`mm read --ids`). Pass
   `--yes` only after the user has confirmed that exact message. Without
   `--yes` and without a terminal it refuses, so an agent always needs it.
5. Write the message as the user would. If the environment has a skill for the
   user's own writing voice, load it before drafting. Chat messages are short.
   A long text belongs in a document, and the chat message carries only the link.

## Targets

Every reading or writing command takes exactly one target:

| Flag | Meaning |
|---|---|
| `-c <channel>` | Channel by **slug** (`town-square`, `dev-backend`), the name in its URL, not its display name. `mm channels` lists them; a DM also has a slug (`<id>__<id>`). |
| `-u <user>` | Direct message with a username (`jane.doe`, no `@`) or an alias (`alex`). |

There is no group DM by username list. To talk to several people, use a channel.

Commands that act on one post accept `--post <ref>`. A ref is a 26-character
post ID or a permalink (`https://chat.acme.com/acme/pl/<post_id>`, what "Copy
link" gives). Without `--post`, they act on **your** last message in the target,
or the Nth one back with `--nth N`. Only your 50 most recent posts are searched.

## Reading

```sh
mm read -c dev-backend -n 30             # oldest → newest
mm read -u alex --json                   # a DM, machine-readable
mm read -u alex --mine --ids             # your own messages there, with post IDs
mm unread                                # what did I miss: mentions first
mm unread --counts                       # one line per channel
mm unread --mentions --json
```

Pass **`--json`** whenever you will parse the output. The text output is for
humans: `[15:04] @jane.doe: text 📎 informe.pdf (1.2 MB)`.

| Command | JSON |
|---|---|
| `mm read --json` | array of `{id, time (RFC3339), from ("@user"), text, own, file_ids?, files?: [{id, name, size (bytes), mime}]}` |
| `mm unread --json` | array of `{channel, type, mentions, unread, truncated, messages: [as above]}`, mentions first. `channel` is a slug for `-c`, or `@username` for `-u` (drop the `@`). `truncated` means there are more unread messages than `-n` returned. |

- `-n` is how many posts are **fetched** (default 20). `--mine` filters after the
  fetch, so raise `-n` to look further back.
- `mm unread` is **read-only**: it never marks anything as read, so the user
  keeps their unread badges on web and mobile. `mm read` does not mark read
  either. Only opening a channel in `mm tui` does.
- System posts (joins, header changes) are skipped by `unread`.
- `mm channels` and `mm users` print text only (`[type] slug`, `@username  Full Name`).

## Writing

```sh
mm send -c dev-backend -m "Deploy listo en staging"
mm send -u alex -m "¿Tienes un momento?"
mm send -u alex -m "El informe" -f ./informe.pdf -f ./captura.png
mm send -u alex -f ./log.txt                       # attachments only, no body
mm edit -u alex -m "Texto corregido"               # your last message there
mm edit --post https://chat.acme.com/acme/pl/4rmsfuwfafyuiq9qkbcgzjg73y -m "…"
mm delete -u alex --nth 2 --yes                    # only after the user confirmed it
```

- Messages are Mattermost Markdown. For a body with quotes, backticks or several
  lines, use a quoted heredoc so the shell leaves it alone:

  ```sh
  mm send -c dev-backend -m "$(cat <<'EOF'
  Fallo en `ingest`: revisad el log adjunto
  EOF
  )" -f ./error.log
  ```

- Up to **10 files** per message; missing files, directories and empty files are
  rejected before anything is uploaded, so a bad path sends nothing.
- Success prints `Message sent.` / `Message sent with N attachments.` /
  `Message edited.` / `Message deleted.`. Verify delivery with `mm read`.
- You can only edit or delete **your own** messages. An edit cannot change
  attachments: to replace a wrong file, delete the message and send it again.
- There are no thread replies yet: every message goes to the channel itself.

## Downloading attachments

```sh
mm download -u alex                       # attachments of alex's latest message that has any
mm download -c dev-backend -o ./in
mm download <post-id|permalink>           # every file of that post
mm download <file-id>                     # one file (ids from `mm read --json` .files[].id)
```

Files go to the desktop download directory (`XDG_DOWNLOAD_DIR`, then
`~/Downloads`, then the current directory) unless `-o` says otherwise. The
directory is created if missing. Each saved file prints
`saved <path> (<size>)`, so read the path from there, not by guessing:

- an existing file is **never overwritten**: a taken name becomes `name (1).ext`;
- the name sent by the server is reduced to its base name, so it cannot write
  outside the target directory;
- each file is written to a temporary `.mm-*.part` first and only takes its real
  name once complete.

The whole file is held in memory while it downloads. Files people send are
untrusted: do not run or open them with an interpreter.

## Scheduling

```sh
mm schedule add -u alex -m "Recuerda la demo" --at "2026-06-09 09:00"
mm schedule add -c dev-backend -m "…" --at "+2h"      # also RFC3339, or "15:04" today
mm schedule list                                       # pending, with ids
mm schedule rm <id>
```

**Delivery is client-side**: the CLI only records the message
(`$XDG_CONFIG_HOME/mm/scheduled.json`), and it is sent by `mm tui` while the TUI
runs. Overdue messages go out when the TUI next starts. Tell the user so: a
scheduled message does not go out at its time unless the TUI is open. Scheduled
messages are text only, with no attachments.

## Aliases

```sh
mm alias list
mm alias add alex alexandra.hernandez     # add or overwrite
mm alias rm alex
```

Aliases work wherever `-u` is accepted. Several aliases may point to one user.
Add one only when the user asks for it.

## MCP server

`mm mcp` serves the same functions over stdio to an MCP client, reusing the saved
session. When the `mm` MCP tools are already available in the session, use
them instead of the CLI. They follow the same rules as above.

| CLI | MCP tool |
|---|---|
| `mm channels` / `mm users` | `list_channels` / `list_users` |
| `mm read` | `read_channel` (`channel` or `user`, `limit`, `mine_only`; returns `post_id` and `files`) |
| `mm unread` | `list_unread` (`limit`, `mentions_only`, `counts_only`) |
| `mm send` | `send_message` (`files` are paths on the host running `mm mcp`) |
| `mm edit` | `edit_message` (`post_id` or `nth`) |
| `mm delete` | `delete_message` (requires `confirm: true`) |
| `mm download` | `download_file` (`file_id`, `post_id` or `channel`/`user`; `dir`) |
| `mm schedule` | `schedule_message`, `manage_scheduled` (`list`, `cancel`) |
| `mm alias` | `manage_alias` (`list`, `add`, `remove`) |
| `mm whoami` | `whoami` |

Resources: `mm://team/channels`, `mm://team/users`, `mm://team/unread`,
`mm://channel/{name}/messages?limit=N`. Prompts: `summarize_channel`,
`draft_reply`, `daily_digest`, `catch_up`.

## Interactive commands: not for agents

`mm login` prompts for a token, and `mm tui` takes over the terminal. Neither
works from an agent's shell. Suggest them to the user instead.

## Errors

Errors go to stderr, prefixed `Error:`, and the exit code is 1.

| Message | Meaning / what to do |
|---|---|
| `missing credentials` / `auth failed` | No session or expired token: the user runs `mm login`. |
| `specify exactly one of channel or user` | Pass `-c` or `-u`, not both, not neither. |
| `user not found` / `channel not found` | Wrong handle or slug: check `mm users`, `mm alias list`, `mm channels`. |
| `no message of yours found in this channel` | `edit`/`delete` without `--post` found none of your posts in the last 50. |
| `… is not a post ID or permalink` | The ref is not 26 characters or a `/pl/` link. |
| `stdin is not a terminal: pass --yes …` | `mm delete` needs `--yes` when run by an agent, after the user's OK. |
| `no message with attachments in the last 50 posts` | `mm download -c/-u` found nothing: give a post or file ID. |
| `too many files` | More than 10 attachments in one message. |
| Mattermost error text (often localised) | The server refused, e.g. editing past its time limit or no permission. Report it as is. |
