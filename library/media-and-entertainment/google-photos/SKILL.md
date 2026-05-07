---
name: pp-google-photos
description: "Printing Press CLI for Google Photos. Google Photos Library and Picker APIs for app-created media, albums, uploads, and user-selected media."
version: "3.9.1"
author: "Cathryn Lavery"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - google-photos-pp-cli
    install:
      - kind: go
        bins: [google-photos-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/media-and-entertainment/google-photos/cmd/google-photos-pp-cli
---

# Google Photos — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `google-photos-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install google-photos --cli-only
   ```
2. Verify: `google-photos-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/google-photos/cmd/google-photos-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## Command Reference

**albums** — Manage app-created Google Photos albums.

- `google-photos-pp-cli albums add-enrichment` — Add text, location, or map enrichment to an app-created album.
- `google-photos-pp-cli albums batch-add-media-items` — Add app-created media items to an app-created album.
- `google-photos-pp-cli albums batch-remove-media-items` — Remove app-created media items from an app-created album.
- `google-photos-pp-cli albums create` — Create an album in the user's Google Photos library.
- `google-photos-pp-cli albums get` — Get an app-created album by ID.
- `google-photos-pp-cli albums list` — List albums created by this app.
- `google-photos-pp-cli albums patch` — Update title or cover photo on an app-created album.

**media-items** — Manage app-created Google Photos media items.

- `google-photos-pp-cli media-items batch-create` — Create media items from upload tokens.
- `google-photos-pp-cli media-items batch-get` — Get multiple app-created media items by ID.
- `google-photos-pp-cli media-items get` — Get an app-created media item by ID.
- `google-photos-pp-cli media-items list` — List media items created by this app.
- `google-photos-pp-cli media-items patch` — Update the description on an app-created media item.
- `google-photos-pp-cli media-items search` — Search app-created media items by album or filters.

**picker** — Create, poll, clean up, and read Google Photos Picker sessions.

- `google-photos-pp-cli picker create-session` — Create a Picker session and return the picker URI.
- `google-photos-pp-cli picker delete-session` — Delete a Picker session after selected media bytes have been retrieved.
- `google-photos-pp-cli picker get-session` — Get Picker session status.
- `google-photos-pp-cli picker list-media-items` — List media items picked by the user during a Picker session.


**Hand-written commands**

- `google-photos-pp-cli upload file <path>` — Upload raw photo or video bytes and print the upload token for media-items batch-create.
- `google-photos-pp-cli picker wait <session-id>` — Poll a Picker session until selected media items are ready.
- `google-photos-pp-cli schema --pretty` — Emit machine-readable command, flag, auth, and safety-policy metadata.
- `google-photos-pp-cli auth list` — List stored OAuth accounts.
- `google-photos-pp-cli auth use <account-email>` — Set the default OAuth account.
- `google-photos-pp-cli auth remove <account-email>` — Remove a stored OAuth account.

## Google Photos Scope Limits

Google Photos Library API read and edit scopes are limited to app-created albums and media. Use Library API commands here for app-created content, uploads, and album/media management. Use Picker commands when a user needs to select media from their broader Google Photos library.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
google-photos-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Authenticate via the browser:

```bash
google-photos-pp-cli auth login --client-id "$GOOGLE_PHOTOS_CLIENT_ID"
```

Tokens are stored locally and refreshed automatically.

For multiple Google accounts, store tokens under account emails and select the account explicitly:

```bash
google-photos-pp-cli auth login you@example.com --client-id "$GOOGLE_PHOTOS_CLIENT_ID"
google-photos-pp-cli auth list --json
google-photos-pp-cli auth use you@example.com
google-photos-pp-cli --account you@example.com albums list --agent
```

Account selection order: `--account`, `GOOGLE_PHOTOS_ACCOUNT`, `auth use` default, then legacy single-token config.

Run `google-photos-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  google-photos-pp-cli albums list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Guarded** — use `--enable-commands` / `--disable-commands` with dotted command paths:

  ```bash
  google-photos-pp-cli --enable-commands albums.list albums list --agent
  google-photos-pp-cli --disable-commands picker.delete-session picker delete-session SESSION_ID --agent
  ```
- **Introspectable** — use `schema --pretty` or `agent-context --pretty` for the full JSON command contract.

### Baked Safety Profiles

Build stronger local guardrails when exposing this CLI or MCP server to autonomous agents:

```bash
make build-readonly
make build-agent-safe
make build-mcp-readonly
make build-mcp-agent-safe
```

`safety_readonly` permits read/list/search/export/schema-style commands and blocks Google Photos mutations. `safety_agent_safe` permits reads and local archive/search workflows while blocking auth writes, uploads, creates, patches, deletes, and album/media mutations.

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
google-photos-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
google-photos-pp-cli feedback --stdin < notes.txt
google-photos-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.google-photos-pp-cli/feedback.jsonl`. They are never POSTed unless `GOOGLE_PHOTOS_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `GOOGLE_PHOTOS_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
google-photos-pp-cli profile save briefing --json
google-photos-pp-cli --profile briefing albums list
google-photos-pp-cli profile list --json
google-photos-pp-cli profile show briefing
google-photos-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `google-photos-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)
## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/google-photos/cmd/google-photos-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add google-photos-pp-mcp -- google-photos-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which google-photos-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   google-photos-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `google-photos-pp-cli <command> --help`.
