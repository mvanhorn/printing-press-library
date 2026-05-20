# Etherpad CLI — Absorb Manifest

## Absorbed (match or beat everything that exists)

| #  | Feature                              | Best Source                                       | Our Implementation                                                       | Added Value                              |
|----|--------------------------------------|---------------------------------------------------|--------------------------------------------------------------------------|------------------------------------------|
| 1  | Pad text get/set                     | rogierlommers/etherpad-cli, python-etherpad_lite  | `etherpad-pp-cli get-text --pad-id`, `set-text --pad-id --text`           | --json + typed exit codes                |
| 2  | Pad HTML get/set                     | python-etherpad_lite, etherpad-lite-client-js     | `get-html`, `set-html` with --rev support                                | --rev for pinned reads                   |
| 3  | Pad create                           | rogierlommers/etherpad-cli, all wrappers          | `create-pad --pad-id --text --author-id`                                 | --dry-run preview                        |
| 4  | Pad delete                           | python-etherpad_lite, etherpad-lite-client-js     | `delete-pad --pad-id --deletion-token`                                   | Token-gated, dry-run                     |
| 5  | Pad copy / move                      | python-etherpad_lite                              | `copy-pad`, `move-pad`, `copy-pad-without-history`                       | All three with --force                   |
| 6  | Pad metadata                         | python-etherpad_lite, etherpad-lite-client-js     | `get-revisions-count`, `get-last-edited`                                 |                                          |
| 7  | Saved revisions                      | python-etherpad_lite                              | `save-revision`, `restore-revision`, `list-saved-revisions`              |                                          |
| 8  | HTML diff between revisions          | python-etherpad_lite                              | `create-diff-html --start-rev --end-rev`                                 |                                          |
| 9  | Group lifecycle                      | python-etherpad_lite, etherpad-lite-client-js     | `create-group`, `list-all-groups`, `delete-group`                        |                                          |
| 10 | Author lifecycle                     | python-etherpad_lite, etherpad-lite-client-js     | `create-author`, `create-author-if-not-exists-for`, `get-author-name`    |                                          |
| 11 | Session lifecycle                    | python-etherpad_lite, etherpad-lite-client-js     | `create-session`, `delete-session`, `get-session-info`                   | TTL-aware                                |
| 12 | Authorship listings                  | python-etherpad_lite                              | `list-authors-of-pad`, `list-pads-of-author`, `list-pads`                |                                          |
| 13 | Per-pad chat                         | python-etherpad_lite                              | `get-chat-history`, `get-chat-head`, `append-chat-message`               |                                          |
| 14 | Public status                        | python-etherpad_lite                              | `get-public-status`, `set-public-status`                                 |                                          |
| 15 | Read-only ID                         | python-etherpad_lite                              | `get-read-only-id`, `get-pad-id`                                         |                                          |
| 16 | Live user counts                     | python-etherpad_lite                              | `pad-users`, `pad-users-count`                                           |                                          |
| 17 | Custom client message                | python-etherpad_lite                              | `send-clients-message --pad-id --msg`                                    |                                          |
| 18 | Token check                          | python-etherpad_lite                              | `check-token` (server-tagged via #7714)                                  |                                          |
| 19 | Server stats                         | (none — added in 1.2.14)                          | `get-stats`                                                              |                                          |
| 20 | Compact pad history                  | (none — added in 1.3.1)                           | `compact-pad --keep-revisions`                                           |                                          |
| 21 | Anonymize author                     | (none — added in 1.3.1)                           | `anonymize-author --author-id`                                           | Compliance/GDPR-friendly                 |
| 22 | Append text                          | (none — added in 1.2.13)                          | `append-text --pad-id --text`                                            |                                          |
| 23 | Get attribute pool / changeset       | (none documented in any wrapper)                  | `get-attribute-pool`, `get-revision-changeset --rev`                     | Required for advanced agents             |
| 24 | OAuth2 auth                          | (none in any CLI/wrapper)                         | `auth login`, `auth status`, `auth logout` (PKCE)                        | Etherpad supports SSO; CLIs ignored it   |
| 25 | Health check                         | (none in any CLI/wrapper)                         | `doctor` — auth + connectivity probe                                     |                                          |
| 26 | JSON output mode                     | (partial)                                         | `--agent` flag on every command produces structured JSON                 | Token-efficient for LLMs                 |

## Transcendence (only possible with our approach)

| # | Feature                       | Command              | Score | Why Only We Can Do This                                                                                                                                                         |
|---|-------------------------------|----------------------|-------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| 1 | Local SQLite mirror           | `sync`               | 9     | HTTP API has no batch query / pagination cursor / "since" filter. Mirroring to FTS5-indexed SQLite turns N round-trips into one query and gives offline analytics no wrapper has.|
| 2 | Cross-pad analytics           | `analytics`          | 9     | API has no aggregate endpoints. Local mirror enables them for free; impossible without it.                                                                                       |
| 3 | Live change tail              | `tail`               | 8     | Etherpad has no webhooks. Polling at a configurable interval and emitting structured events is the only agent-friendly way to watch a pad — we ship it as one command.           |
| 4 | Compound archive workflow     | `workflow archive`   | 8     | Walks groups → pads → authors → chats and writes everything to local store in one command. Migration/backup is otherwise N scripts; we collapse it.                              |
| 5 | Bulk JSONL import             | `import`             | 7     | Read JSONL (or stdin) and call the appropriate create/upsert endpoint per record. Migrations from other notes/wiki tools become a single command.                                |
| 6 | Agent context bootstrap       | `agent-context`      | 8     | Emit JSON describing the entire CLI surface, auth state, and connectivity in one shot — designed to brief an LLM at session start. Token-efficient onboarding nothing else has.   |

## What we explicitly do NOT cover (and why)

- **Live operational-transform / cursor positions** — Out of scope for an HTTP-based CLI. Use [ep_ai_mcp](https://github.com/ether/ep_ai_mcp) inside Etherpad for changeset-aware tools; or socket.io directly for real-time co-editing.
- **Per-paragraph authorship & text provenance** — These need access to the in-process attribute pool. ep_ai_mcp ships them; the OpenAPI surface doesn't expose them.
- **Plugin install/uninstall** — Lives at the admin API (`/admin/openapi.json`), separate surface, separate auth model, separate audit needed.
